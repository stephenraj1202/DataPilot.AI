package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/finops-platform/shared/config"
	"github.com/gin-gonic/gin"
	stripe "github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	stripeinvoice "github.com/stripe/stripe-go/v76/invoice"
	"github.com/stripe/stripe-go/v76/subscriptionitem"
)

// CheckoutHandler creates Stripe Checkout Sessions for paid plans.
type CheckoutHandler struct {
	DB         *sql.DB
	SuccessURL string
	CancelURL  string
	Cfg        *config.Config
}

type createCheckoutRequest struct {
	PlanName string `json:"plan_name" binding:"required"`
}

// CreateCheckoutSession creates a Stripe Checkout Session and returns the redirect URL.
// POST /billing/checkout
func (h *CheckoutHandler) CreateCheckoutSession(c *gin.Context) {
	accountID := c.GetHeader("X-Account-ID")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_id required"})
		return
	}

	// Look up the account owner email from DB
	var userEmail string
	_ = h.DB.QueryRow(
		`SELECT u.email FROM users u
		 JOIN user_roles ur ON ur.user_id = u.id
		 JOIN roles r ON r.id = ur.role_id
		 WHERE u.account_id = ? AND r.name = 'account_owner'
		 LIMIT 1`, accountID,
	).Scan(&userEmail)
	if userEmail == "" {
		// fallback: any user in the account
		_ = h.DB.QueryRow(`SELECT email FROM users WHERE account_id = ? LIMIT 1`, accountID).Scan(&userEmail)
	}

	var req createCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get plan details
	plan, err := GetPlanByName(h.DB, req.PlanName)
	if err != nil || plan.PriceCents == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or free plan"})
		return
	}

	if plan.StripePriceID == "" {
		// No Stripe price configured — activate locally and return a sentinel URL
		// so the frontend can redirect to the success page directly.
		if activateErr := h.activateLocalSubscription(accountID, plan); activateErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to activate plan: " + activateErr.Error()})
			return
		}
		// Refresh UBB streams so they pick up the new plan
		h.refreshUBBStreams(accountID)
		c.JSON(http.StatusOK, gin.H{
			"checkout_url": h.SuccessURL + "?local=1&plan=" + plan.Name,
			"local":        true,
		})
		return
	}

	// Get or create Stripe customer for this account
	stripeCustomerID, err := h.getOrCreateStripeCustomer(accountID, userEmail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get/create Stripe customer: " + err.Error()})
		return
	}

	// Create Stripe Checkout Session
	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(stripeCustomerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(plan.StripePriceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(h.SuccessURL + "?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(h.CancelURL),
		Metadata: map[string]string{
			"account_id": accountID,
			"plan_name":  req.PlanName,
		},
	}

	sess, err := session.New(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create checkout session: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"checkout_url": sess.URL})
}

// ConfirmCheckoutSession retrieves a completed Stripe Checkout Session and activates the subscription.
// POST /billing/checkout/confirm
// Called by the frontend success page as a reliable fallback when webhooks are delayed/failing.
func (h *CheckoutHandler) ConfirmCheckoutSession(c *gin.Context) {
	accountID := c.GetHeader("X-Account-ID")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_id required"})
		return
	}

	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Retrieve session from Stripe to verify payment
	params := &stripe.CheckoutSessionParams{}
	params.AddExpand("subscription")
	params.AddExpand("subscription.items.data.price")
	sess, err := session.Get(req.SessionID, params)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session: " + err.Error()})
		return
	}

	if sess.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid &&
		sess.PaymentStatus != stripe.CheckoutSessionPaymentStatusNoPaymentRequired {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "payment not completed"})
		return
	}

	if sess.Subscription == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no subscription in session"})
		return
	}

	sub := sess.Subscription
	if len(sub.Items.Data) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no subscription items"})
		return
	}

	priceID := sub.Items.Data[0].Price.ID

	// Resolve plan by stripe_price_id
	var planID, planName string
	_ = h.DB.QueryRow(
		`SELECT id, name FROM subscription_plans WHERE stripe_price_id = ?`, priceID,
	).Scan(&planID, &planName)

	// Fallback: match by amount
	if planID == "" && sess.AmountTotal > 0 {
		_ = h.DB.QueryRow(
			`SELECT id, name FROM subscription_plans WHERE price_cents = ? AND price_cents > 0 LIMIT 1`,
			sess.AmountTotal,
		).Scan(&planID, &planName)
	}

	if planID == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not resolve plan for price %s", priceID)})
		return
	}

	// Deactivate old subscriptions and upsert the new one
	_, _ = h.DB.Exec(
		`UPDATE stripe_subscriptions SET status='canceled', deleted_at=NOW()
		 WHERE account_id = ? AND stripe_subscription_id != ? AND deleted_at IS NULL`,
		accountID, sub.ID,
	)

	_, err = h.DB.Exec(
		`INSERT INTO stripe_subscriptions
		 (id, account_id, stripe_subscription_id, plan_id, status, current_period_start, current_period_end)
		 VALUES (UUID(), ?, ?, ?, ?, FROM_UNIXTIME(?), FROM_UNIXTIME(?))
		 ON DUPLICATE KEY UPDATE
		   plan_id=VALUES(plan_id), status=VALUES(status),
		   current_period_start=VALUES(current_period_start),
		   current_period_end=VALUES(current_period_end),
		   deleted_at=NULL`,
		accountID, sub.ID, planID, string(sub.Status),
		sub.CurrentPeriodStart, sub.CurrentPeriodEnd,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to activate subscription"})
		return
	}

	// Sync invoices from Stripe for this customer so they appear immediately in the UI
	h.syncInvoicesFromStripe(accountID, sub.Customer.ID)

	// Refresh UBB streams so they pick up the new Stripe subscription items
	h.refreshUBBStreams(accountID)

	c.JSON(http.StatusOK, gin.H{"plan": planName, "status": string(sub.Status)})
}

func (h *CheckoutHandler) getOrCreateStripeCustomer(accountID, email string) (string, error) {
	var stripeCustomerID string
	err := h.DB.QueryRow(
		`SELECT stripe_customer_id FROM stripe_customers WHERE account_id = ?`,
		accountID,
	).Scan(&stripeCustomerID)

	if err == nil {
		return stripeCustomerID, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	// Create new Stripe customer
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Metadata: map[string]string{
			"account_id": accountID,
		},
	}
	c, err := customer.New(params)
	if err != nil {
		return "", err
	}

	// Store in DB
	_, err = h.DB.Exec(
		`INSERT INTO stripe_customers (id, account_id, stripe_customer_id, email) VALUES (UUID(), ?, ?, ?)`,
		accountID, c.ID, email,
	)
	if err != nil {
		return "", err
	}

	return c.ID, nil
}

// syncInvoicesFromStripe pulls all invoices for a Stripe customer and saves them to the DB.
func (h *CheckoutHandler) syncInvoicesFromStripe(accountID, stripeCustomerID string) {
	params := &stripe.InvoiceListParams{
		Customer: stripe.String(stripeCustomerID),
	}
	params.Filters.AddFilter("limit", "", "100")

	iter := stripeinvoice.List(params)
	for iter.Next() {
		inv := iter.Invoice()
		if inv.AmountDue == 0 {
			continue
		}
		pdfURL := inv.InvoicePDF
		_, err := h.DB.Exec(
			`INSERT INTO stripe_invoices
			 (id, account_id, stripe_invoice_id, amount_cents, currency, status, invoice_pdf_url)
			 VALUES (UUID(), ?, ?, ?, ?, ?, ?)
			 ON DUPLICATE KEY UPDATE
			   status=VALUES(status), invoice_pdf_url=VALUES(invoice_pdf_url)`,
			accountID, inv.ID, inv.AmountDue, string(inv.Currency), string(inv.Status), pdfURL,
		)
		if err != nil {
			log.Printf("syncInvoices: failed to save invoice %s: %v", inv.ID, err)
		}
	}
	if err := iter.Err(); err != nil {
		log.Printf("syncInvoices: stripe list error: %v", err)
	}
}

// activateLocalSubscription creates/updates a local subscription record for plans
// that don't have a Stripe price ID configured yet.
func (h *CheckoutHandler) activateLocalSubscription(accountID string, plan *Plan) error {
	// Cancel any existing active subscriptions
	_, _ = h.DB.Exec(
		`UPDATE stripe_subscriptions SET status='canceled', deleted_at=NOW()
		 WHERE account_id=? AND deleted_at IS NULL`,
		accountID,
	)
	_, err := h.DB.Exec(
		`INSERT INTO stripe_subscriptions
		 (id, account_id, stripe_subscription_id, plan_id, status, current_period_start, current_period_end)
		 VALUES (UUID(), ?, ?, ?, 'active', NOW(), DATE_ADD(NOW(), INTERVAL 30 DAY))`,
		accountID, "local_"+plan.Name+"_"+accountID, plan.ID,
	)
	return err
}

// refreshUBBStreams updates all active UBB streams for an account to reflect the
// current Stripe subscription items. Called after checkout/plan change.
func (h *CheckoutHandler) refreshUBBStreams(accountID string) {
	// Get current Stripe subscription ID
	var stripeSubID, stripeCustomerID string
	_ = h.DB.QueryRow(
		`SELECT ss.stripe_subscription_id, sc.stripe_customer_id
		 FROM stripe_subscriptions ss
		 LEFT JOIN stripe_customers sc ON sc.account_id = ss.account_id
		 WHERE ss.account_id=? AND ss.status IN ('active','trialing') AND ss.deleted_at IS NULL
		 ORDER BY ss.created_at DESC LIMIT 1`,
		accountID,
	).Scan(&stripeSubID, &stripeCustomerID)

	if stripeSubID == "" || isLocalSubID(stripeSubID) {
		// Local plan — just update plan_name on streams
		var planName string
		_ = h.DB.QueryRow(
			`SELECT sp.name FROM stripe_subscriptions ss
			 JOIN subscription_plans sp ON sp.id=ss.plan_id
			 WHERE ss.account_id=? AND ss.status IN ('active','trialing') AND ss.deleted_at IS NULL
			 ORDER BY ss.created_at DESC LIMIT 1`,
			accountID,
		).Scan(&planName)
		if planName != "" {
			_, _ = h.DB.Exec(
				`UPDATE ubb_streams SET plan_name=? WHERE account_id=? AND deleted_at IS NULL`,
				planName, accountID,
			)
		}
		return
	}

	// Stripe subscription — find metered sub items and assign to streams
	rows, err := h.DB.Query(
		`SELECT id FROM ubb_streams WHERE account_id=? AND deleted_at IS NULL`,
		accountID,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	var streamIDs []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			streamIDs = append(streamIDs, id)
		}
	}
	if len(streamIDs) == 0 {
		return
	}

	// Collect available metered sub items
	params := &stripe.SubscriptionItemListParams{
		Subscription: stripe.String(stripeSubID),
	}
	iter := subscriptionitem.List(params)
	var meteredItems []string
	for iter.Next() {
		si := iter.SubscriptionItem()
		if si.Price != nil && si.Price.Recurring != nil &&
			si.Price.Recurring.UsageType == stripe.PriceRecurringUsageTypeMetered {
			meteredItems = append(meteredItems, si.ID)
		}
	}

	// Get plan name
	var planName string
	_ = h.DB.QueryRow(
		`SELECT sp.name FROM stripe_subscriptions ss
		 JOIN subscription_plans sp ON sp.id=ss.plan_id
		 WHERE ss.account_id=? AND ss.status IN ('active','trialing') AND ss.deleted_at IS NULL
		 ORDER BY ss.created_at DESC LIMIT 1`,
		accountID,
	).Scan(&planName)

	// Assign metered items to streams (one per stream)
	for i, streamID := range streamIDs {
		subItemID := ""
		if i < len(meteredItems) {
			subItemID = meteredItems[i]
		}
		_, _ = h.DB.Exec(
			`UPDATE ubb_streams SET stripe_sub_item_id=?, stripe_customer_id=?, plan_name=?
			 WHERE id=? AND account_id=? AND deleted_at IS NULL`,
			subItemID, stripeCustomerID, planName, streamID, accountID,
		)
	}
	log.Printf("refreshUBBStreams: updated %d streams for account %s (plan=%s, meteredItems=%d)",
		len(streamIDs), accountID, planName, len(meteredItems))
}
