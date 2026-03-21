package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RecommendationScheduler generates optimization recommendations daily at 2 AM UTC.
type RecommendationScheduler struct {
	DB *sql.DB
}

// Start launches the background goroutine that runs recommendations at 2 AM UTC daily.
func (r *RecommendationScheduler) Start() {
	go func() {
		for {
			now := time.Now().UTC()
			next2AM := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, time.UTC)
			if now.After(next2AM) {
				next2AM = next2AM.Add(24 * time.Hour)
			}
			time.Sleep(time.Until(next2AM))
			r.generateRecommendations()
		}
	}()
}

// generateRecommendations identifies idle/oversized resources and unattached storage.
func (r *RecommendationScheduler) generateRecommendations() {
	log.Println("[recommendations] Generating cost optimization recommendations")

	rows, err := r.DB.Query(
		`SELECT id, account_id FROM cloud_accounts WHERE deleted_at IS NULL AND status = 'active'`,
	)
	if err != nil {
		log.Printf("[recommendations] Failed to query cloud accounts: %v", err)
		return
	}
	defer rows.Close()

	type acctRow struct {
		ID        string
		AccountID string
	}
	var accounts []acctRow
	for rows.Next() {
		var a acctRow
		if err := rows.Scan(&a.ID, &a.AccountID); err == nil {
			accounts = append(accounts, a)
		}
	}

	for _, acct := range accounts {
		r.generateForAccount(acct.ID, acct.AccountID)
	}

	log.Println("[recommendations] Recommendation generation complete")
}

func (r *RecommendationScheduler) generateForAccount(cloudAccountID, accountID string) {
	// Clear existing recommendations for this account (regenerate daily)
	_, _ = r.DB.Exec(
		`DELETE FROM cost_recommendations WHERE cloud_account_id = ?`,
		cloudAccountID,
	)

	r.detectIdleResources(cloudAccountID)
	r.detectOversizedResources(cloudAccountID)
	r.detectUnattachedStorage(cloudAccountID)
}

// detectIdleResources finds resources with zero cost for 7 consecutive days.
func (r *RecommendationScheduler) detectIdleResources(cloudAccountID string) {
	cutoff := time.Now().UTC().AddDate(0, 0, -7).Format("2006-01-02")

	rows, err := r.DB.Query(
		`SELECT resource_id, service_name, AVG(cost_amount) as avg_cost
		 FROM cloud_costs
		 WHERE cloud_account_id = ? AND date >= ? AND cost_amount = 0
		 GROUP BY resource_id, service_name
		 HAVING COUNT(DISTINCT date) >= 7`,
		cloudAccountID, cutoff,
	)
	if err != nil {
		log.Printf("[recommendations] idle query error: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var resourceID, serviceName string
		var avgCost float64
		if err := rows.Scan(&resourceID, &serviceName, &avgCost); err != nil {
			continue
		}
		r.insertRecommendation(cloudAccountID, "idle_resource", resourceID, serviceName,
			"Resource has had 0% usage for 7 consecutive days", 0)
	}
}

// detectOversizedResources finds resources with low average cost over 30 days (proxy for utilization).
func (r *RecommendationScheduler) detectOversizedResources(cloudAccountID string) {
	cutoff := time.Now().UTC().AddDate(0, 0, -30).Format("2006-01-02")

	rows, err := r.DB.Query(
		`SELECT resource_id, service_name,
		        AVG(cost_amount) as avg_daily_cost,
		        MAX(cost_amount) as max_daily_cost
		 FROM cloud_costs
		 WHERE cloud_account_id = ? AND date >= ? AND cost_amount > 0
		 GROUP BY resource_id, service_name
		 HAVING COUNT(DISTINCT date) >= 20
		    AND (AVG(cost_amount) / MAX(cost_amount)) < 0.20`,
		cloudAccountID, cutoff,
	)
	if err != nil {
		log.Printf("[recommendations] oversized query error: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var resourceID, serviceName string
		var avgCost, maxCost float64
		if err := rows.Scan(&resourceID, &serviceName, &avgCost, &maxCost); err != nil {
			continue
		}
		// Potential savings: difference between max and avg, annualized to monthly
		monthlySavings := (maxCost - avgCost) * 30
		r.insertRecommendation(cloudAccountID, "oversized_resource", resourceID, serviceName,
			"Resource average utilization is below 20% over the past 30 days", monthlySavings)
	}
}

// detectUnattachedStorage finds storage resources that incur cost but appear unattached
// (heuristic: service name contains "storage" or "volume" and cost is low but non-zero).
func (r *RecommendationScheduler) detectUnattachedStorage(cloudAccountID string) {
	cutoff := time.Now().UTC().AddDate(0, 0, -30).Format("2006-01-02")

	rows, err := r.DB.Query(
		`SELECT resource_id, service_name, SUM(cost_amount) as total_cost
		 FROM cloud_costs
		 WHERE cloud_account_id = ? AND date >= ?
		   AND (LOWER(service_name) LIKE '%storage%' OR LOWER(service_name) LIKE '%volume%'
		        OR LOWER(service_name) LIKE '%disk%' OR LOWER(service_name) LIKE '%ebs%')
		   AND cost_amount > 0
		 GROUP BY resource_id, service_name
		 HAVING COUNT(DISTINCT date) >= 20`,
		cloudAccountID, cutoff,
	)
	if err != nil {
		log.Printf("[recommendations] unattached storage query error: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var resourceID, serviceName string
		var totalCost float64
		if err := rows.Scan(&resourceID, &serviceName, &totalCost); err != nil {
			continue
		}
		monthlySavings := totalCost / 30 * 30 // normalize to monthly
		r.insertRecommendation(cloudAccountID, "unattached_storage", resourceID, serviceName,
			"Storage resource may be unattached and incurring unnecessary costs", monthlySavings)
	}
}

func (r *RecommendationScheduler) insertRecommendation(
	cloudAccountID, recType, resourceID, serviceName, description string,
	monthlySavings float64,
) {
	id := uuid.New().String()
	_, err := r.DB.Exec(
		`INSERT INTO cost_recommendations
		   (id, cloud_account_id, recommendation_type, resource_id, service_name, description, potential_monthly_savings, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NOW())`,
		id, cloudAccountID, recType, resourceID, serviceName, description, monthlySavings,
	)
	if err != nil {
		log.Printf("[recommendations] Failed to insert recommendation: %v", err)
	}
}

// RecommendationHandler handles recommendation HTTP endpoints.
type RecommendationHandler struct {
	DB *sql.DB
}

// GetRecommendations handles GET /finops/recommendations
func (h *RecommendationHandler) GetRecommendations(c *gin.Context) {
	accountID := c.GetString("account_id")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	rows, err := h.DB.Query(
		`SELECT cr.id, cr.cloud_account_id, cr.recommendation_type, cr.resource_id,
		        cr.service_name, cr.description, cr.potential_monthly_savings, cr.created_at
		 FROM cost_recommendations cr
		 JOIN cloud_accounts ca ON ca.id = cr.cloud_account_id
		 WHERE ca.account_id = ? AND ca.deleted_at IS NULL
		 ORDER BY cr.potential_monthly_savings DESC`,
		accountID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query recommendations"})
		return
	}
	defer rows.Close()

	type recRow struct {
		ID                      string  `json:"id"`
		CloudAccountID          string  `json:"cloud_account_id"`
		Type                    string  `json:"type"`
		ResourceID              string  `json:"resource_id"`
		ServiceName             string  `json:"service_name"`
		Description             string  `json:"description"`
		PotentialMonthlySavings float64 `json:"potential_monthly_savings"`
		CreatedAt               string  `json:"created_at"`
	}

	var recs []recRow
	var totalSavings float64
	for rows.Next() {
		var rec recRow
		if err := rows.Scan(
			&rec.ID, &rec.CloudAccountID, &rec.Type, &rec.ResourceID,
			&rec.ServiceName, &rec.Description, &rec.PotentialMonthlySavings, &rec.CreatedAt,
		); err != nil {
			continue
		}
		totalSavings += rec.PotentialMonthlySavings
		recs = append(recs, rec)
	}

	c.JSON(http.StatusOK, gin.H{
		"recommendations":         recs,
		"total_potential_savings": totalSavings,
	})
}
