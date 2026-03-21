package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// CostHandler handles cost aggregation queries.
type CostHandler struct {
	DB *sql.DB
}

// GetCostSummary handles GET /finops/costs/summary
// Query params: start_date (YYYY-MM-DD), end_date (YYYY-MM-DD)
func (h *CostHandler) GetCostSummary(c *gin.Context) {
	accountID := c.GetString("account_id")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	startDate := c.DefaultQuery("start_date", "")
	endDate := c.DefaultQuery("end_date", "")

	// Default to current month if no dates provided
	if startDate == "" {
		startDate = time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}

	// Build date filter clause — always applied
	dateFilter := " AND cc.date BETWEEN ? AND ?"
	args := []any{accountID, startDate, endDate}

	// Total cost
	var totalCost float64
	err := h.DB.QueryRow(
		`SELECT COALESCE(SUM(cc.cost_amount), 0)
		 FROM cloud_costs cc
		 JOIN cloud_accounts ca ON ca.id = cc.cloud_account_id
		 WHERE ca.account_id = ? AND ca.deleted_at IS NULL`+dateFilter,
		args...,
	).Scan(&totalCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query total cost"})
		return
	}

	// Breakdown by provider
	providerRows, err := h.DB.Query(
		`SELECT ca.provider, COALESCE(SUM(cc.cost_amount), 0) as total
		 FROM cloud_costs cc
		 JOIN cloud_accounts ca ON ca.id = cc.cloud_account_id
		 WHERE ca.account_id = ? AND ca.deleted_at IS NULL`+dateFilter+`
		 GROUP BY ca.provider`,
		args...,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query provider breakdown"})
		return
	}
	defer providerRows.Close()

	byProvider := map[string]float64{}
	for providerRows.Next() {
		var provider string
		var total float64
		if err := providerRows.Scan(&provider, &total); err == nil {
			byProvider[provider] = total
		}
	}

	// Breakdown by service with provider info — no limit, all services
	serviceRows, err := h.DB.Query(
		`SELECT cc.service_name, ca.provider, COALESCE(SUM(cc.cost_amount), 0) as total
		 FROM cloud_costs cc
		 JOIN cloud_accounts ca ON ca.id = cc.cloud_account_id
		 WHERE ca.account_id = ? AND ca.deleted_at IS NULL`+dateFilter+`
		 GROUP BY cc.service_name, ca.provider
		 ORDER BY total DESC`,
		args...,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query service breakdown"})
		return
	}
	defer serviceRows.Close()

	type serviceEntry struct {
		Service  string  `json:"service"`
		Provider string  `json:"provider"`
		Cost     float64 `json:"cost"`
	}
	var byService []serviceEntry
	serviceCountByProvider := map[string]int{}
	for serviceRows.Next() {
		var e serviceEntry
		if err := serviceRows.Scan(&e.Service, &e.Provider, &e.Cost); err == nil {
			byService = append(byService, e)
			serviceCountByProvider[e.Provider]++
		}
	}

	// Daily breakdown
	dailyRows, err := h.DB.Query(
		`SELECT cc.date, COALESCE(SUM(cc.cost_amount), 0) as total
		 FROM cloud_costs cc
		 JOIN cloud_accounts ca ON ca.id = cc.cloud_account_id
		 WHERE ca.account_id = ? AND ca.deleted_at IS NULL`+dateFilter+`
		 GROUP BY cc.date
		 ORDER BY cc.date ASC`,
		args...,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query daily breakdown"})
		return
	}
	defer dailyRows.Close()

	type dailyEntry struct {
		Date string  `json:"date"`
		Cost float64 `json:"cost"`
	}
	var daily []dailyEntry
	for dailyRows.Next() {
		var e dailyEntry
		if err := dailyRows.Scan(&e.Date, &e.Cost); err == nil {
			daily = append(daily, e)
		}
	}

	// Monthly trends (last 12 months, ignoring the date filter)
	monthlyRows, err := h.DB.Query(
		`SELECT DATE_FORMAT(cc.date, '%Y-%m') as month, COALESCE(SUM(cc.cost_amount), 0) as total
		 FROM cloud_costs cc
		 JOIN cloud_accounts ca ON ca.id = cc.cloud_account_id
		 WHERE ca.account_id = ? AND ca.deleted_at IS NULL
		   AND cc.date >= DATE_SUB(CURDATE(), INTERVAL 12 MONTH)
		 GROUP BY month
		 ORDER BY month ASC`,
		accountID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query monthly trends"})
		return
	}
	defer monthlyRows.Close()

	type monthlyEntry struct {
		Month string  `json:"month"`
		Cost  float64 `json:"cost"`
	}
	var monthly []monthlyEntry
	for monthlyRows.Next() {
		var e monthlyEntry
		if err := monthlyRows.Scan(&e.Month, &e.Cost); err == nil {
			monthly = append(monthly, e)
		}
	}

	// Forecast: project end-of-month spend based on daily average so far this month.
	// Uses the current MTD spend divided by elapsed days × total days in month.
	forecastCost := totalCost
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0) // first day of next month
	totalDaysInMonth := int(monthEnd.Sub(monthStart).Hours() / 24)
	elapsedDays := int(now.Sub(monthStart).Hours()/24) + 1 // +1 to include today
	if elapsedDays > 0 && elapsedDays < totalDaysInMonth {
		dailyAvg := totalCost / float64(elapsedDays)
		forecastCost = dailyAvg * float64(totalDaysInMonth) // full precision, no rounding
	}

	c.JSON(http.StatusOK, gin.H{
		"total_cost":                totalCost,
		"forecast_cost":             forecastCost,
		"currency":                  "USD",
		"breakdown_by_provider":     byProvider,
		"breakdown_by_service":      byService,
		"service_count_by_provider": serviceCountByProvider,
		"daily_costs":               daily,
		"monthly_trends":            monthly,
	})
}
