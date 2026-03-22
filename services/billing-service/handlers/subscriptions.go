package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	stripe "github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/subscription"
)

// SubscriptionHandler handles subscription creation and modification.
type SubscriptionHandler struct {
	DB             *sql.DB
	RazorpayKeyID  string
	RazorpaySecret string
}

type createSubscriptionRequest struct {
	AccountID       string `json:"account_id" binding:"required"`
	Plan            string `json:"plan" binding:"required"` // free, base, pro, enterprise
	PaymentMethodID string `json:"payment_method_id"`
}

type updateSubscriptionRequest struct {
	AccountID string `json:"account_id" binding:"required"`
	NewPlan   string `json:"new_plan" binding:"required"`
}

// Subscribe creates a new Stripe subscription for an account.
// POST /billing/subscribe
func (h *SubscriptionHandler) Subscribe(c *gin.Context) {
	var req createSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get plan details
	plan, err := GetPlanByName(h.DB, req.Plan)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan: " + req.Plan})
		return
	}

	// Get Stripe customer ID for this account
	var stripeCustomerID string
	err = h.DB.QueryRow(
		`SELECT stripe_customer_id FROM stripe_customers WHERE account_id = ?`,
		req.AccountID,
	).Scan(&stripeCustomerID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "no Stripe customer found for this account"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	// Cancel existing active subscription if any
	var existingStripeSubID string
	_ = h.DB.QueryRow(
		`SELECT stripe_subscription_id FROM stripe_subscriptions
		 WHERE account_id = ? AND status IN ('active','trialing') AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`,
		req.AccountID,
	).Scan(&existingStripeSubID)

	if existingStripeSubID != "" && !isLocalSubID(existingStripeSubID) {
		_, _ = subscription.Cancel(existingStripeSubID, nil)
	}

	// Mark old subscription as canceled in DB
	if existingStripeSubID != "" {
		_, _ = h.DB.Exec(
			`UPDATE stripe_subscriptions SET status='canceled', deleted_at=NOW()
			 WHERE account_id = ? AND stripe_subscription_id = ?`,
			req.AccountID, existingStripeSubID,
		)
	}

	var stripeSub *stripe.Subscription
	subID := uuid.New().String()

	if plan.StripePriceID != "" {
		params := &stripe.SubscriptionParams{
			Customer: stripe.String(stripeCustomerID),
			Items: []*stripe.SubscriptionItemsParams{
				{Price: stripe.String(plan.StripePriceID)},
			},
		}
		if req.PaymentMethodID != "" {
			params.DefaultPaymentMethod = stripe.String(req.PaymentMethodID)
		}
		stripeSub, err = subscription.New(params)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create Stripe subscription: " + err.Error()})
			return
		}

		_, err = h.DB.Exec(
			`INSERT INTO stripe_subscriptions
			 (id, account_id, stripe_subscription_id, plan_id, status, current_period_start, current_period_end)
			 VALUES (?, ?, ?, ?, ?, FROM_UNIXTIME(?), FROM_UNIXTIME(?))`,
			subID, req.AccountID, stripeSub.ID, plan.ID, string(stripeSub.Status),
			stripeSub.CurrentPeriodStart, stripeSub.CurrentPeriodEnd,
		)
	} else {
		// Free plan - local record only
		_, err = h.DB.Exec(
			`INSERT INTO stripe_subscriptions
			 (id, account_id, stripe_subscription_id, plan_id, status, current_period_start, current_period_end)
			 VALUES (?, ?, ?, ?, 'active', NOW(), DATE_ADD(NOW(), INTERVAL 30 DAY))`,
			subID, req.AccountID, "free_"+req.AccountID, plan.ID,
		)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store subscription"})
		return
	}

	logAuditEvent(h.DB, "", req.AccountID, "subscription_created", "stripe_subscription", subID, c.ClientIP(), c.Request.UserAgent())

	resp := gin.H{
		"subscription_id": subID,
		"plan":            req.Plan,
		"status":          "active",
	}
	if stripeSub != nil {
		resp["stripe_subscription_id"] = stripeSub.ID
		resp["current_period_end"] = stripeSub.CurrentPeriodEnd
	}

	c.JSON(http.StatusCreated, resp)
}

// UpdateSubscription upgrades or downgrades a subscription.
// PUT /billing/subscription
func (h *SubscriptionHandler) UpdateSubscription(c *gin.Context) {
	var req updateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get new plan details
	newPlan, err := GetPlanByName(h.DB, req.NewPlan)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan: " + req.NewPlan})
		return
	}

	// Get current subscription
	var currentSubID, stripeSubID, currentPlanID string
	err = h.DB.QueryRow(
		`SELECT id, stripe_subscription_id, plan_id FROM stripe_subscriptions
		 WHERE account_id = ? AND status IN ('active','trialing') AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`,
		req.AccountID,
	).Scan(&currentSubID, &stripeSubID, &currentPlanID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active subscription found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	// Get current plan price to determine upgrade vs downgrade
	var currentPriceCents int
	_ = h.DB.QueryRow(`SELECT price_cents FROM subscription_plans WHERE id = ?`, currentPlanID).Scan(&currentPriceCents)

	isUpgrade := newPlan.PriceCents > currentPriceCents
	effectiveDate := "immediate"
	if !isUpgrade {
		effectiveDate = "period_end"
	}

	var prorationAmount int64

	if newPlan.StripePriceID != "" && !isLocalSubID(stripeSubID) {
		// Update Stripe subscription
		params := &stripe.SubscriptionParams{}

		if isUpgrade {
			// Upgrade: apply immediately with proration
			params.ProrationBehavior = stripe.String("create_prorations")
			params.Items = []*stripe.SubscriptionItemsParams{
				{Price: stripe.String(newPlan.StripePriceID)},
			}
		} else {
			// Downgrade: apply at period end (no proration)
			params.ProrationBehavior = stripe.String("none")
			params.CancelAtPeriodEnd = stripe.Bool(false)
			params.BillingCycleAnchorUnchanged = stripe.Bool(true)
			params.Items = []*stripe.SubscriptionItemsParams{
				{Price: stripe.String(newPlan.StripePriceID)},
			}
		}

		updatedSub, err := subscription.Update(stripeSubID, params)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update Stripe subscription: " + err.Error()})
			return
		}

		if isUpgrade {
			// Update DB immediately
			_, _ = h.DB.Exec(
				`UPDATE stripe_subscriptions SET plan_id=?, status=?, current_period_start=FROM_UNIXTIME(?), current_period_end=FROM_UNIXTIME(?)
				 WHERE id=?`,
				newPlan.ID, string(updatedSub.Status),
				updatedSub.CurrentPeriodStart, updatedSub.CurrentPeriodEnd,
				currentSubID,
			)
		} else {
			// Mark cancel_at_period_end for downgrade
			_, _ = h.DB.Exec(
				`UPDATE stripe_subscriptions SET cancel_at_period_end=TRUE WHERE id=?`,
				currentSubID,
			)
		}
	} else {
		// Local subscription (free plan or no Stripe price ID)
		if isUpgrade {
			_, _ = h.DB.Exec(
				`UPDATE stripe_subscriptions SET plan_id=? WHERE id=?`,
				newPlan.ID, currentSubID,
			)
		} else {
			_, _ = h.DB.Exec(
				`UPDATE stripe_subscriptions SET plan_id=?, cancel_at_period_end=TRUE WHERE id=?`,
				newPlan.ID, currentSubID,
			)
		}
	}

	logAuditEvent(h.DB, "", req.AccountID, "subscription_updated", "stripe_subscription", currentSubID, c.ClientIP(), c.Request.UserAgent())

	c.JSON(http.StatusOK, gin.H{
		"subscription_id":  currentSubID,
		"new_plan":         req.NewPlan,
		"proration_amount": prorationAmount,
		"effective_date":   effectiveDate,
	})
}

// isLocalSubID returns true if the subscription ID is a locally-generated one (not from Stripe).
// Covers: free_, local_, trial_ (free trial), rzp_ (Razorpay), and any UUID-style local IDs.
func isLocalSubID(id string) bool {
	return strings.HasPrefix(id, "free_") ||
		strings.HasPrefix(id, "local_") ||
		strings.HasPrefix(id, "trial_") ||
		strings.HasPrefix(id, "rzp_")
}
