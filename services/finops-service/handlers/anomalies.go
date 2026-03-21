package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AnomalyDetector detects cost anomalies and manages the cost_anomalies table.
type AnomalyDetector struct {
	DB       *sql.DB
	EmailCfg EmailConfig
}

// AnomalyHandler handles anomaly HTTP endpoints.
type AnomalyHandler struct {
	DB *sql.DB
}

// DetectAnomalies calculates 30-day moving average baselines and creates anomaly records.
// It is called after each cost sync.
func (d *AnomalyDetector) DetectAnomalies() {
	log.Println("[anomaly] Running anomaly detection")

	// Get all active cloud accounts
	rows, err := d.DB.Query(
		`SELECT id, account_id FROM cloud_accounts WHERE deleted_at IS NULL AND status = 'active'`,
	)
	if err != nil {
		log.Printf("[anomaly] Failed to query cloud accounts: %v", err)
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

	today := time.Now().UTC().Truncate(24 * time.Hour)
	yesterday := today.AddDate(0, 0, -1)

	for _, acct := range accounts {
		d.detectForAccount(acct.ID, acct.AccountID, yesterday)
	}
}

func (d *AnomalyDetector) detectForAccount(cloudAccountID, accountID string, date time.Time) {
	// Calculate 30-day moving average (days -31 to -1 relative to date)
	windowEnd := date.AddDate(0, 0, -1)
	windowStart := date.AddDate(0, 0, -31)

	var baseline float64
	err := d.DB.QueryRow(
		`SELECT COALESCE(AVG(daily_total), 0)
		 FROM (
		   SELECT date, SUM(cost_amount) as daily_total
		   FROM cloud_costs
		   WHERE cloud_account_id = ? AND date BETWEEN ? AND ?
		   GROUP BY date
		 ) t`,
		cloudAccountID,
		windowStart.Format("2006-01-02"),
		windowEnd.Format("2006-01-02"),
	).Scan(&baseline)
	if err != nil || baseline == 0 {
		return // Not enough data
	}

	// Get actual cost for the target date
	var actualCost float64
	err = d.DB.QueryRow(
		`SELECT COALESCE(SUM(cost_amount), 0) FROM cloud_costs
		 WHERE cloud_account_id = ? AND date = ?`,
		cloudAccountID, date.Format("2006-01-02"),
	).Scan(&actualCost)
	if err != nil || actualCost == 0 {
		return
	}

	deviation := ((actualCost - baseline) / baseline) * 100
	if deviation < 20 {
		return // No anomaly
	}

	severity := classifySeverity(deviation)

	// Identify contributing services (top services by cost on that day)
	serviceRows, err := d.DB.Query(
		`SELECT service_name, SUM(cost_amount) as total
		 FROM cloud_costs
		 WHERE cloud_account_id = ? AND date = ?
		 GROUP BY service_name
		 ORDER BY total DESC
		 LIMIT 5`,
		cloudAccountID, date.Format("2006-01-02"),
	)
	if err != nil {
		log.Printf("[anomaly] Failed to query contributing services: %v", err)
		return
	}
	defer serviceRows.Close()

	var services []string
	for serviceRows.Next() {
		var svc string
		var total float64
		if err := serviceRows.Scan(&svc, &total); err == nil {
			services = append(services, svc)
		}
	}

	servicesJSON, _ := json.Marshal(services)

	// Upsert anomaly record (avoid duplicates for same account+date)
	anomalyID := uuid.New().String()
	_, err = d.DB.Exec(
		`INSERT INTO cost_anomalies
		   (id, cloud_account_id, date, baseline_cost, actual_cost, deviation_percentage, severity, contributing_services, acknowledged, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, FALSE, NOW())
		 ON DUPLICATE KEY UPDATE
		   baseline_cost = VALUES(baseline_cost),
		   actual_cost = VALUES(actual_cost),
		   deviation_percentage = VALUES(deviation_percentage),
		   severity = VALUES(severity),
		   contributing_services = VALUES(contributing_services)`,
		anomalyID, cloudAccountID, date.Format("2006-01-02"),
		baseline, actualCost, deviation, severity, string(servicesJSON),
	)
	if err != nil {
		log.Printf("[anomaly] Failed to insert anomaly: %v", err)
		return
	}

	log.Printf("[anomaly] Detected %s anomaly for account %s on %s (%.1f%% deviation)",
		severity, cloudAccountID, date.Format("2006-01-02"), deviation)

	// Send notification
	d.sendAnomalyNotification(accountID, cloudAccountID, date, baseline, actualCost, deviation, severity, services)
}

func classifySeverity(deviation float64) string {
	switch {
	case deviation > 60:
		return "high"
	case deviation > 40:
		return "medium"
	default:
		return "low"
	}
}

func (d *AnomalyDetector) sendAnomalyNotification(
	accountID, cloudAccountID string,
	date time.Time,
	baseline, actual, deviation float64,
	severity string,
	services []string,
) {
	// Fetch Account_Owner and Admin emails for this account
	rows, err := d.DB.Query(
		`SELECT DISTINCT u.email
		 FROM users u
		 JOIN user_roles ur ON ur.user_id = u.id
		 JOIN roles r ON r.id = ur.role_id
		 WHERE u.account_id = ? AND r.name IN ('account_owner', 'admin') AND u.deleted_at IS NULL`,
		accountID,
	)
	if err != nil {
		log.Printf("[anomaly] Failed to query notification recipients: %v", err)
		return
	}
	defer rows.Close()

	var recipients []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err == nil {
			recipients = append(recipients, email)
		}
	}

	if len(recipients) == 0 {
		return
	}

	subject := fmt.Sprintf("[FinOps Alert] %s cost anomaly detected on %s", severity, date.Format("2006-01-02"))
	body := fmt.Sprintf(
		"A cost anomaly has been detected.\n\nDate: %s\nSeverity: %s\nBaseline: $%.2f\nActual: $%.2f\nDeviation: %.1f%%\nContributing services: %v\n\nPlease review your cloud costs.",
		date.Format("2006-01-02"), severity, baseline, actual, deviation, services,
	)

	if err := sendEmail(d.EmailCfg, recipients, subject, body); err != nil {
		log.Printf("[anomaly] Failed to send notification email: %v", err)
	}
}

// ListAnomalies handles GET /finops/anomalies
func (h *AnomalyHandler) ListAnomalies(c *gin.Context) {
	accountID := c.GetString("account_id")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	days := c.DefaultQuery("days", "30")

	rows, err := h.DB.Query(
		`SELECT ca.id, ca.cloud_account_id, ca.date, ca.baseline_cost, ca.actual_cost,
		        ca.deviation_percentage, ca.severity, ca.contributing_services,
		        ca.acknowledged, ca.acknowledged_by, ca.acknowledged_at, ca.created_at
		 FROM cost_anomalies ca
		 JOIN cloud_accounts cla ON cla.id = ca.cloud_account_id
		 WHERE cla.account_id = ? AND ca.date >= DATE_SUB(NOW(), INTERVAL ? DAY)
		 ORDER BY ca.date DESC, ca.deviation_percentage DESC`,
		accountID, days,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query anomalies"})
		return
	}
	defer rows.Close()

	type anomalyRow struct {
		ID                   string   `json:"id"`
		CloudAccountID       string   `json:"cloud_account_id"`
		Date                 string   `json:"date"`
		BaselineCost         float64  `json:"baseline_cost"`
		ActualCost           float64  `json:"actual_cost"`
		DeviationPercentage  float64  `json:"deviation_percentage"`
		Severity             string   `json:"severity"`
		ContributingServices []string `json:"contributing_services"`
		Acknowledged         bool     `json:"acknowledged"`
		AcknowledgedBy       *string  `json:"acknowledged_by"`
		AcknowledgedAt       *string  `json:"acknowledged_at"`
		CreatedAt            string   `json:"created_at"`
	}

	var anomalies []anomalyRow
	for rows.Next() {
		var a anomalyRow
		var servicesJSON []byte
		if err := rows.Scan(
			&a.ID, &a.CloudAccountID, &a.Date, &a.BaselineCost, &a.ActualCost,
			&a.DeviationPercentage, &a.Severity, &servicesJSON,
			&a.Acknowledged, &a.AcknowledgedBy, &a.AcknowledgedAt, &a.CreatedAt,
		); err != nil {
			continue
		}
		_ = json.Unmarshal(servicesJSON, &a.ContributingServices)
		anomalies = append(anomalies, a)
	}

	c.JSON(http.StatusOK, gin.H{"anomalies": anomalies})
}

// AcknowledgeAnomaly handles POST /finops/anomalies/:id/acknowledge
func (h *AnomalyHandler) AcknowledgeAnomaly(c *gin.Context) {
	accountID := c.GetString("account_id")
	userID := c.GetString("user_id")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	anomalyID := c.Param("id")

	// Verify the anomaly belongs to this account
	var count int
	err := h.DB.QueryRow(
		`SELECT COUNT(*) FROM cost_anomalies ca
		 JOIN cloud_accounts cla ON cla.id = ca.cloud_account_id
		 WHERE ca.id = ? AND cla.account_id = ?`,
		anomalyID, accountID,
	).Scan(&count)
	if err != nil || count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "anomaly not found"})
		return
	}

	_, err = h.DB.Exec(
		`UPDATE cost_anomalies SET acknowledged = TRUE, acknowledged_by = ?, acknowledged_at = NOW()
		 WHERE id = ?`,
		userID, anomalyID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to acknowledge anomaly"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "anomaly acknowledged"})
}
