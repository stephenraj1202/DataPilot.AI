package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetSubscription returns the active subscription for the authenticated account.
// GET /billing/subscription  (called via api-gateway with X-Account-ID header)
func (h *SubscriptionHandler) GetSubscription(c *gin.Context) {
	accountID := c.GetHeader("X-Account-ID")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_id required"})
		return
	}

	type planOut struct {
		ID                     string   `json:"id"`
		Name                   string   `json:"name"`
		PriceCents             int      `json:"price_cents"`
		MaxCloudAccounts       *int     `json:"max_cloud_accounts"`
		MaxDatabaseConnections *int     `json:"max_database_connections"`
		RateLimitPerMinute     int      `json:"rate_limit_per_minute"`
		Features               []string `json:"features"`
	}
	type subOut struct {
		ID                 string  `json:"id"`
		Plan               planOut `json:"plan"`
		Status             string  `json:"status"`
		CurrentPeriodStart string  `json:"current_period_start"`
		CurrentPeriodEnd   string  `json:"current_period_end"`
		CancelAtPeriodEnd  bool    `json:"cancel_at_period_end"`
	}

	var s subOut
	var featuresJSON string
	err := h.DB.QueryRow(`
		SELECT ss.id, ss.status,
		       DATE_FORMAT(ss.current_period_start, '%Y-%m-%dT%H:%i:%sZ'),
		       DATE_FORMAT(ss.current_period_end, '%Y-%m-%dT%H:%i:%sZ'),
		       ss.cancel_at_period_end,
		       sp.id, sp.name, sp.price_cents,
		       sp.max_cloud_accounts, sp.max_database_connections, sp.rate_limit_per_minute,
		       COALESCE(sp.features,'[]')
		FROM stripe_subscriptions ss
		JOIN subscription_plans sp ON sp.id = ss.plan_id
		WHERE ss.account_id = ? AND ss.status IN ('active','trialing') AND ss.deleted_at IS NULL
		ORDER BY ss.created_at DESC LIMIT 1`, accountID,
	).Scan(
		&s.ID, &s.Status, &s.CurrentPeriodStart, &s.CurrentPeriodEnd, &s.CancelAtPeriodEnd,
		&s.Plan.ID, &s.Plan.Name, &s.Plan.PriceCents,
		&s.Plan.MaxCloudAccounts, &s.Plan.MaxDatabaseConnections, &s.Plan.RateLimitPerMinute,
		&featuresJSON,
	)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active subscription"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	_ = json.Unmarshal([]byte(featuresJSON), &s.Plan.Features)
	c.JSON(http.StatusOK, gin.H{"subscription": s})
}

// GetPlans returns all available subscription plans.
// GET /billing/plans
func GetPlans(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(`
			SELECT id, name, price_cents, COALESCE(stripe_price_id,''),
			       max_cloud_accounts, max_database_connections, rate_limit_per_minute,
			       COALESCE(features,'[]')
			FROM subscription_plans ORDER BY price_cents ASC`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer rows.Close()

		type planOut struct {
			ID                     string   `json:"id"`
			Name                   string   `json:"name"`
			PriceCents             int      `json:"price_cents"`
			StripePriceID          string   `json:"stripe_price_id"`
			MaxCloudAccounts       *int     `json:"max_cloud_accounts"`
			MaxDatabaseConnections *int     `json:"max_database_connections"`
			RateLimitPerMinute     int      `json:"rate_limit_per_minute"`
			Features               []string `json:"features"`
		}

		var plans []planOut
		for rows.Next() {
			var p planOut
			var featuresJSON string
			if err := rows.Scan(&p.ID, &p.Name, &p.PriceCents, &p.StripePriceID,
				&p.MaxCloudAccounts, &p.MaxDatabaseConnections, &p.RateLimitPerMinute,
				&featuresJSON); err != nil {
				continue
			}
			_ = json.Unmarshal([]byte(featuresJSON), &p.Features)
			plans = append(plans, p)
		}
		c.JSON(http.StatusOK, gin.H{"plans": plans})
	}
}
