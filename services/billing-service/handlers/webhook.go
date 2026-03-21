package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	stripe "github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/webhook"
)

// WebhookHandler handles Stripe webhook events.
type WebhookHandler struct {
	DB            *sql.DB
	WebhookSecret string
	EmailCfg      EmailConfig
}

// HandleWebhook processes incoming Stripe webhook events.
// POST /billing/webhook
func (h *WebhookHandler) HandleWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	sigHeader := c.GetHeader("Stripe-Signature")
	event, err := webhook.ConstructEventWithOptions(body, sigHeader, h.WebhookSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true},
	)
	if err != nil {
		log.Printf("Webhook signature verification failed: %v", err)
		logAuditEvent(h.DB, "", "", "webhook_signature_failed", "stripe_webhook", "", c.ClientIP(), c.Request.UserAgent())
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook signature"})
		return
	}

	// Idempotency check
	var count int
	_ = h.DB.QueryRow(
		`SELECT COUNT(*) FROM audit_logs WHERE action_type = ? AND resource_id = ?`,
		"webhook_processed", event.ID,
	).Scan(&count)
	if count > 0 {
		c.JSON(http.StatusOK, gin.H{"status": "already processed"})
		return
	}

	done := make(chan error, 1)
	go func() { done <- h.processEvent(event) }()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("Error processing webhook event %s: %v", event.ID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "event processing failed"})
			return
		}
	case <-time.After(5 * time.Second):
		log.Printf("Webhook event %s processing timed out", event.ID)
		c.JSON(http.StatusOK, gin.H{"status": "accepted"})
		return
	}

	logAuditEvent(h.DB, "", "", "webhook_processed", "stripe_webhook", event.ID, c.ClientIP(), c.Request.UserAgent())
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *WebhookHandler) processEvent(event stripe.Event) error {
	switch event.Type {
	case "checkout.session.completed":
		return h.handleCheckoutSessionCompleted(event)
	case "customer.subscription.created":
		return h.handleSubscriptionCreated(event)
	case "customer.subscription.updated":
		return h.handleSubscriptionUpdated(event)
	case "customer.subscription.deleted":
		return h.handleSubscriptionDeleted(event)
	case "invoice.payment_succeeded":
		return h.handleInvoicePaymentSucceeded(event)
	case "invoice.payment_failed":
		return h.handleInvoicePaymentFailed(event)
	default:
		log.Printf("Unhandled webhook event type: %s", event.Type)
	}
	return nil
}

func (h *WebhookHandler) handleCheckoutSessionCompleted(event stripe.Event) error {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		return err
	}

	accountID := ""
	if sess.Metadata != nil {
		accountID = sess.Metadata["account_id"]
	}
	if accountID == "" && sess.Customer != nil {
		accountID = h.getAccountIDByStripeCustomer(sess.Customer.ID)
	}
	if accountID == "" {
		return fmt.Errorf("no account found for checkout session %s", sess.ID)
	}
	if sess.Subscription == nil {
		return nil
	}

	subID := sess.Subscription.ID

	// Resolve plan: prefer metadata plan_name, then match by price_cents
	planName := ""
	if sess.Metadata != nil {
		planName = sess.Metadata["plan_name"]
	}

	var planID string
	if planName != "" {
		_ = h.DB.QueryRow(`SELECT id FROM subscription_plans WHERE name = ?`, planName).Scan(&planID)
	}
	if planID == "" && sess.AmountTotal > 0 {
		_ = h.DB.QueryRow(
			`SELECT id FROM subscription_plans WHERE price_cents = ? AND price_cents > 0 LIMIT 1`,
			sess.AmountTotal,
		).Scan(&planID)
	}
	if planID == "" {
		log.Printf("checkout.session.completed: could not resolve plan for session %s", sess.ID)
		return nil
	}

	return h.upsertActiveSubscription(accountID, subID, planID, "active", 0, 0)
}

func (h *WebhookHandler) handleSubscriptionCreated(event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return err
	}

	accountID := h.getAccountIDByStripeCustomer(sub.Customer.ID)
	if accountID == "" {
		return fmt.Errorf("no account found for Stripe customer %s", sub.Customer.ID)
	}

	planID := h.getPlanIDByStripePrice(sub.Items.Data[0].Price.ID)
	return h.upsertActiveSubscription(accountID, sub.ID, planID, string(sub.Status), sub.CurrentPeriodStart, sub.CurrentPeriodEnd)
}

func (h *WebhookHandler) handleSubscriptionUpdated(event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return err
	}

	planID := h.getPlanIDByStripePrice(sub.Items.Data[0].Price.ID)

	_, err := h.DB.Exec(
		`UPDATE stripe_subscriptions
		 SET plan_id=?, status=?, current_period_start=FROM_UNIXTIME(?), current_period_end=FROM_UNIXTIME(?),
		     cancel_at_period_end=?, deleted_at=NULL
		 WHERE stripe_subscription_id=?`,
		planID, string(sub.Status),
		sub.CurrentPeriodStart, sub.CurrentPeriodEnd,
		sub.CancelAtPeriodEnd,
		sub.ID,
	)
	return err
}

func (h *WebhookHandler) handleSubscriptionDeleted(event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return err
	}

	_, err := h.DB.Exec(
		`UPDATE stripe_subscriptions SET status='canceled', deleted_at=NOW()
		 WHERE stripe_subscription_id=?`,
		sub.ID,
	)
	return err
}

// upsertActiveSubscription cancels all other active subs for the account, then inserts/updates the new one.
func (h *WebhookHandler) upsertActiveSubscription(accountID, stripeSubID, planID, status string, periodStart, periodEnd int64) error {
	// Deactivate all other subscriptions for this account
	_, _ = h.DB.Exec(
		`UPDATE stripe_subscriptions
		 SET status='canceled', deleted_at=NOW()
		 WHERE account_id = ? AND stripe_subscription_id != ? AND deleted_at IS NULL`,
		accountID, stripeSubID,
	)

	id := uuid.New().String()
	if periodStart > 0 {
		_, err := h.DB.Exec(
			`INSERT INTO stripe_subscriptions
			 (id, account_id, stripe_subscription_id, plan_id, status, current_period_start, current_period_end)
			 VALUES (?, ?, ?, ?, ?, FROM_UNIXTIME(?), FROM_UNIXTIME(?))
			 ON DUPLICATE KEY UPDATE
			   plan_id=VALUES(plan_id), status=VALUES(status),
			   current_period_start=VALUES(current_period_start),
			   current_period_end=VALUES(current_period_end),
			   deleted_at=NULL`,
			id, accountID, stripeSubID, planID, status, periodStart, periodEnd,
		)
		return err
	}

	_, err := h.DB.Exec(
		`INSERT INTO stripe_subscriptions
		 (id, account_id, stripe_subscription_id, plan_id, status, current_period_start, current_period_end)
		 VALUES (?, ?, ?, ?, ?, NOW(), DATE_ADD(NOW(), INTERVAL 30 DAY))
		 ON DUPLICATE KEY UPDATE
		   plan_id=VALUES(plan_id), status=VALUES(status), deleted_at=NULL`,
		id, accountID, stripeSubID, planID, status,
	)
	return err
}

func (h *WebhookHandler) handleInvoicePaymentSucceeded(event stripe.Event) error {
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return err
	}

	accountID := h.getAccountIDByStripeCustomer(inv.Customer.ID)
	if accountID == "" {
		return fmt.Errorf("no account found for Stripe customer %s", inv.Customer.ID)
	}

	if err := h.upsertInvoice(accountID, &inv); err != nil {
		return err
	}

	// Send invoice email notification
	go func() {
		subject := fmt.Sprintf("Payment received - FinOps Platform")
		body := fmt.Sprintf("Your payment of %.2f %s has been received.\n\nThank you for using FinOps Platform.",
			float64(inv.AmountDue)/100.0, string(inv.Currency))
		var ownerEmail string
		_ = h.DB.QueryRow(
			`SELECT u.email FROM users u
			 JOIN user_roles ur ON ur.user_id = u.id
			 JOIN roles r ON r.id = ur.role_id
			 WHERE u.account_id = ? AND r.name = 'account_owner' LIMIT 1`, accountID,
		).Scan(&ownerEmail)
		if ownerEmail != "" {
			_ = sendEmail(h.EmailCfg, []string{ownerEmail}, subject, body)
		}
	}()

	return nil
}

func (h *WebhookHandler) handleInvoicePaymentFailed(event stripe.Event) error {
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return err
	}

	accountID := h.getAccountIDByStripeCustomer(inv.Customer.ID)
	if accountID == "" {
		return fmt.Errorf("no account found for Stripe customer %s", inv.Customer.ID)
	}

	if err := h.upsertInvoice(accountID, &inv); err != nil {
		return err
	}

	h.sendPaymentFailureNotifications(accountID, inv.Customer.Email)
	return nil
}

func (h *WebhookHandler) upsertInvoice(accountID string, inv *stripe.Invoice) error {
	id := uuid.New().String()
	pdfURL := ""
	if inv.InvoicePDF != "" {
		pdfURL = inv.InvoicePDF
	}

	_, err := h.DB.Exec(
		`INSERT INTO stripe_invoices
		 (id, account_id, stripe_invoice_id, amount_cents, currency, status, invoice_pdf_url)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		   status=VALUES(status), invoice_pdf_url=VALUES(invoice_pdf_url)`,
		id, accountID, inv.ID, inv.AmountDue, string(inv.Currency), string(inv.Status), pdfURL,
	)
	return err
}

func (h *WebhookHandler) getAccountIDByStripeCustomer(stripeCustomerID string) string {
	var accountID string
	_ = h.DB.QueryRow(
		`SELECT account_id FROM stripe_customers WHERE stripe_customer_id = ?`,
		stripeCustomerID,
	).Scan(&accountID)
	return accountID
}

func (h *WebhookHandler) getPlanIDByStripePrice(stripePriceID string) string {
	var planID string
	_ = h.DB.QueryRow(
		`SELECT id FROM subscription_plans WHERE stripe_price_id = ?`,
		stripePriceID,
	).Scan(&planID)
	return planID
}

func (h *WebhookHandler) sendPaymentFailureNotifications(accountID, customerEmail string) {
	var ownerEmail string
	_ = h.DB.QueryRow(
		`SELECT u.email FROM users u
		 JOIN user_roles ur ON ur.user_id = u.id
		 JOIN roles r ON r.id = ur.role_id
		 WHERE u.account_id = ? AND r.name = 'account_owner'
		 LIMIT 1`,
		accountID,
	).Scan(&ownerEmail)

	if ownerEmail == "" && customerEmail != "" {
		ownerEmail = customerEmail
	}

	subject := "Payment Failed - FinOps Platform"
	body := fmt.Sprintf(`Your subscription payment has failed. Please update your payment method.

Account ID: %s

Log in to update your payment information.

FinOps Platform Team`, accountID)

	var recipients []string
	if ownerEmail != "" {
		recipients = append(recipients, ownerEmail)
	}
	if h.EmailCfg.SuperAdminEmail != "" {
		recipients = append(recipients, h.EmailCfg.SuperAdminEmail)
	}
	if len(recipients) > 0 {
		if err := sendEmail(h.EmailCfg, recipients, subject, body); err != nil {
			log.Printf("Failed to send payment failure notification: %v", err)
		}
	}

	logAuditEvent(h.DB, "", accountID, "payment_failed_notification", "stripe_invoice", "", "", "")
}
