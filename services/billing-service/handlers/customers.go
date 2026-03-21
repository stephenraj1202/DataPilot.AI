package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	stripe "github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/subscription"
)

// CustomerHandler handles Stripe customer creation.
type CustomerHandler struct {
	DB *sql.DB
}

type createCustomerRequest struct {
	AccountID string `json:"account_id" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
}

// CreateCustomer creates a Stripe customer and assigns the Free plan.
// POST /billing/customers
func (h *CustomerHandler) CreateCustomer(c *gin.Context) {
	var req createCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if customer already exists for this account
	var existingID string
	err := h.DB.QueryRow(
		`SELECT stripe_customer_id FROM stripe_customers WHERE account_id = ?`,
		req.AccountID,
	).Scan(&existingID)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "customer already exists for this account", "stripe_customer_id": existingID})
		return
	}
	if err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	// Create Stripe customer
	params := &stripe.CustomerParams{
		Email: stripe.String(req.Email),
		Metadata: map[string]string{
			"account_id": req.AccountID,
		},
	}
	stripeCustomer, err := customer.New(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create Stripe customer: " + err.Error()})
		return
	}

	// Store in database
	id := uuid.New().String()
	_, err = h.DB.Exec(
		`INSERT INTO stripe_customers (id, account_id, stripe_customer_id, email) VALUES (?, ?, ?, ?)`,
		id, req.AccountID, stripeCustomer.ID, req.Email,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store customer"})
		return
	}

	// Assign Free plan subscription (no payment method needed for $0 plan)
	freePlanID, err := GetFreePlanID(h.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve free plan"})
		return
	}

	var stripePriceID string
	_ = h.DB.QueryRow(`SELECT COALESCE(stripe_price_id,'') FROM subscription_plans WHERE id = ?`, freePlanID).Scan(&stripePriceID)

	// Only create Stripe subscription if there's a price ID configured
	if stripePriceID != "" {
		subParams := &stripe.SubscriptionParams{
			Customer: stripe.String(stripeCustomer.ID),
			Items: []*stripe.SubscriptionItemsParams{
				{Price: stripe.String(stripePriceID)},
			},
		}
		stripeSub, err := subscription.New(subParams)
		if err == nil {
			subID := uuid.New().String()
			_, _ = h.DB.Exec(
				`INSERT INTO stripe_subscriptions
				 (id, account_id, stripe_subscription_id, plan_id, status, current_period_start, current_period_end)
				 VALUES (?, ?, ?, ?, ?, FROM_UNIXTIME(?), FROM_UNIXTIME(?))`,
				subID, req.AccountID, stripeSub.ID, freePlanID, string(stripeSub.Status),
				stripeSub.CurrentPeriodStart, stripeSub.CurrentPeriodEnd,
			)
		}
	} else {
		// Create a local free subscription record without Stripe
		subID := uuid.New().String()
		_, _ = h.DB.Exec(
			`INSERT INTO stripe_subscriptions
			 (id, account_id, stripe_subscription_id, plan_id, status, current_period_start, current_period_end)
			 VALUES (?, ?, ?, ?, 'active', NOW(), DATE_ADD(NOW(), INTERVAL 30 DAY))`,
			subID, req.AccountID, "free_"+req.AccountID, freePlanID,
		)
	}

	logAuditEvent(h.DB, "", req.AccountID, "customer_created", "stripe_customer", id, c.ClientIP(), c.Request.UserAgent())

	c.JSON(http.StatusCreated, gin.H{
		"id":                 id,
		"account_id":         req.AccountID,
		"stripe_customer_id": stripeCustomer.ID,
		"email":              req.Email,
		"plan":               "free",
	})
}
