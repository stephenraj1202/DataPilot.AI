package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	razorpay "github.com/razorpay/razorpay-go"
)

// GetSubscription returns the active subscription for the authenticated account.
// For Razorpay subscriptions, enriches with live data from Razorpay API.
// GET /billing/subscription
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
		// Razorpay live fields (only set for Razorpay subscriptions)
		RazorpaySubID    string `json:"razorpay_sub_id,omitempty"`
		NextChargeAt     string `json:"next_charge_at,omitempty"`
		NextChargeAmount int    `json:"next_charge_amount,omitempty"` // paise
		RemainingCount   int    `json:"remaining_count,omitempty"`
		PaidCount        int    `json:"paid_count,omitempty"`
		RazorpayStatus   string `json:"razorpay_status,omitempty"`
	}

	var s subOut
	var featuresJSON string
	var rawSubID string
	err := h.DB.QueryRow(`
		SELECT ss.id, ss.stripe_subscription_id, ss.status,
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
		&s.ID, &rawSubID, &s.Status, &s.CurrentPeriodStart, &s.CurrentPeriodEnd, &s.CancelAtPeriodEnd,
		&s.Plan.ID, &s.Plan.Name, &s.Plan.PriceCents,
		&s.Plan.MaxCloudAccounts, &s.Plan.MaxDatabaseConnections, &s.Plan.RateLimitPerMinute,
		&featuresJSON,
	)
	if err == sql.ErrNoRows {
		// No subscription row — return synthetic free plan
		var freePlan planOut
		var freeFeaturesJSON string
		freePlanErr := h.DB.QueryRow(
			`SELECT id, name, price_cents, max_cloud_accounts, max_database_connections, rate_limit_per_minute, COALESCE(features,'[]')
			 FROM subscription_plans WHERE name='free' LIMIT 1`,
		).Scan(&freePlan.ID, &freePlan.Name, &freePlan.PriceCents,
			&freePlan.MaxCloudAccounts, &freePlan.MaxDatabaseConnections, &freePlan.RateLimitPerMinute,
			&freeFeaturesJSON)
		if freePlanErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "no active subscription"})
			return
		}
		_ = json.Unmarshal([]byte(freeFeaturesJSON), &freePlan.Features)
		now := time.Now()
		c.JSON(http.StatusOK, gin.H{"subscription": subOut{
			ID:                 "free_" + accountID,
			Plan:               freePlan,
			Status:             "active",
			CurrentPeriodStart: now.Format("2006-01-02T15:04:05Z"),
			CurrentPeriodEnd:   now.AddDate(0, 1, 0).Format("2006-01-02T15:04:05Z"),
			CancelAtPeriodEnd:  false,
		}})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	_ = json.Unmarshal([]byte(featuresJSON), &s.Plan.Features)

	// ── Enrich with live Razorpay data when sub ID is a Razorpay subscription ──
	// Razorpay subscription IDs start with "sub_"
	if strings.HasPrefix(rawSubID, "sub_") && h.RazorpayKeyID != "" {
		s.RazorpaySubID = rawSubID
		client := razorpay.NewClient(h.RazorpayKeyID, h.RazorpaySecret)
		rzpSub, fetchErr := client.Subscription.Fetch(rawSubID, nil, nil)
		if fetchErr == nil {
			// Status from Razorpay
			if rzpStatus, ok := rzpSub["status"].(string); ok {
				s.RazorpayStatus = rzpStatus
				// Map Razorpay status to our status
				switch rzpStatus {
				case "active", "authenticated":
					s.Status = "active"
				case "halted", "cancelled", "expired", "completed":
					s.Status = rzpStatus
				}
			}

			// charge_at = next billing timestamp (unix)
			if chargeAt, ok := toInt64(rzpSub["charge_at"]); ok && chargeAt > 0 {
				t := time.Unix(chargeAt, 0)
				s.NextChargeAt = t.Format("2006-01-02T15:04:05Z")
				s.CurrentPeriodEnd = t.Format("2006-01-02T15:04:05Z")
				// Update DB with live period end
				_, _ = h.DB.Exec(
					`UPDATE stripe_subscriptions SET current_period_end=FROM_UNIXTIME(?), updated_at=NOW()
					 WHERE stripe_subscription_id=? AND deleted_at IS NULL`,
					chargeAt, rawSubID,
				)
			}

			// current_start from Razorpay
			if startAt, ok := toInt64(rzpSub["current_start"]); ok && startAt > 0 {
				s.CurrentPeriodStart = time.Unix(startAt, 0).Format("2006-01-02T15:04:05Z")
			}

			// Next charge amount = plan amount from Razorpay plan
			if planData, ok := rzpSub["plan"].(map[string]interface{}); ok {
				if item, ok := planData["item"].(map[string]interface{}); ok {
					if amt, ok := toInt64(item["amount"]); ok {
						s.NextChargeAmount = int(amt) // paise
					}
				}
			}
			// Fallback: use our plan price
			if s.NextChargeAmount == 0 {
				s.NextChargeAmount = s.Plan.PriceCents
			}

			// Paid / remaining counts
			if paid, ok := toInt64(rzpSub["paid_count"]); ok {
				s.PaidCount = int(paid)
			}
			if remaining, ok := toInt64(rzpSub["remaining_count"]); ok {
				s.RemainingCount = int(remaining)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"subscription": s})
}

// toInt64 safely converts interface{} (float64 from JSON or int) to int64.
func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	}
	return 0, false
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
