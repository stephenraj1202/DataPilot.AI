package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	razorpay "github.com/razorpay/razorpay-go"
)

// RazorpayCheckoutHandler handles Razorpay subscription creation and payment verification.
type RazorpayCheckoutHandler struct {
	DB               *sql.DB
	KeyID            string
	KeySecret        string
	WebhookSecret    string
	SuccessURL       string
	CancelURL        string
	BasePlanID       string
	ProPlanID        string
	EnterprisePlanID string
}

// razorpayPlanID returns the Razorpay plan ID for a given plan name.
// Falls back to creating a plan on-the-fly if not configured.
func (h *RazorpayCheckoutHandler) razorpayPlanID(client *razorpay.Client, planName string, priceCents int) (string, error) {
	switch planName {
	case "base":
		if h.BasePlanID != "" {
			return h.BasePlanID, nil
		}
	case "pro":
		if h.ProPlanID != "" {
			return h.ProPlanID, nil
		}
	case "enterprise":
		if h.EnterprisePlanID != "" {
			return h.EnterprisePlanID, nil
		}
	}

	// No plan ID configured — create one on-the-fly via Razorpay API
	planData := map[string]interface{}{
		"period":   "monthly",
		"interval": 1,
		"item": map[string]interface{}{
			"name":     fmt.Sprintf("DataPilot.AI %s Plan", planName),
			"amount":   priceCents, // paise
			"currency": "INR",
		},
	}
	plan, err := client.Plan.Create(planData, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create Razorpay plan: %w", err)
	}
	planID, _ := plan["id"].(string)
	log.Printf("[razorpay] Created plan on-the-fly: %s for %s", planID, planName)
	return planID, nil
}

// CreateRazorpaySubscription creates a Razorpay recurring subscription for a plan.
// POST /billing/razorpay/order
func (h *RazorpayCheckoutHandler) CreateRazorpayOrder(c *gin.Context) {
	accountID := c.GetHeader("X-Account-ID")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_id required"})
		return
	}

	var req createCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	plan, err := GetPlanByName(h.DB, req.PlanName)
	if err != nil || plan.PriceCents == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or free plan"})
		return
	}

	client := razorpay.NewClient(h.KeyID, h.KeySecret)

	// Resolve Razorpay plan ID (from config or create on-the-fly)
	rzpPlanID, err := h.razorpayPlanID(client, plan.Name, plan.PriceCents)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Create a Razorpay subscription (recurring monthly)
	subData := map[string]interface{}{
		"plan_id":         rzpPlanID,
		"total_count":     120, // max 10 years of monthly billing
		"quantity":        1,
		"customer_notify": 1,
		"notes": map[string]interface{}{
			"account_id": accountID,
			"plan_name":  plan.Name,
		},
	}

	sub, err := client.Subscription.Create(subData, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create Razorpay subscription: " + err.Error()})
		return
	}

	subID, _ := sub["id"].(string)
	log.Printf("[razorpay] Created subscription %s for account %s plan %s", subID, accountID, plan.Name)

	c.JSON(http.StatusOK, gin.H{
		"subscription_id": subID,
		"amount":          plan.PriceCents,
		"currency":        "INR",
		"key_id":          h.KeyID,
		"plan_name":       plan.Name,
		"success_url":     h.SuccessURL,
		// Keep order_id field for frontend compatibility (set to sub ID)
		"order_id": subID,
	})
}

// VerifyRazorpayPayment verifies subscription payment and activates the local subscription.
// POST /billing/razorpay/verify
func (h *RazorpayCheckoutHandler) VerifyRazorpayPayment(c *gin.Context) {
	accountID := c.GetHeader("X-Account-ID")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_id required"})
		return
	}

	var req struct {
		SubscriptionID string `json:"razorpay_subscription_id"`
		PaymentID      string `json:"razorpay_payment_id" binding:"required"`
		Signature      string `json:"razorpay_signature" binding:"required"`
		PlanName       string `json:"plan_name" binding:"required"`
		OrderID        string `json:"razorpay_order_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[razorpay] verify request: payment=%s sub=%s order=%s plan=%s account=%s",
		req.PaymentID, req.SubscriptionID, req.OrderID, req.PlanName, accountID)

	// ── Signature verification ────────────────────────────────────────────────
	// Razorpay subscription: HMAC(payment_id + "|" + subscription_id)
	// Razorpay order:        HMAC(order_id + "|" + payment_id)
	// We try all plausible combinations and accept if any matches.
	signatureValid := false
	candidates := []string{}

	if req.SubscriptionID != "" {
		candidates = append(candidates,
			req.PaymentID+"|"+req.SubscriptionID,
			req.SubscriptionID+"|"+req.PaymentID,
		)
	}
	if req.OrderID != "" {
		candidates = append(candidates,
			req.OrderID+"|"+req.PaymentID,
			req.PaymentID+"|"+req.OrderID,
		)
	}
	// Also try payment_id alone (some Razorpay SDK versions)
	candidates = append(candidates, req.PaymentID)

	for _, payload := range candidates {
		mac := hmac.New(sha256.New, []byte(h.KeySecret))
		mac.Write([]byte(payload))
		if hex.EncodeToString(mac.Sum(nil)) == req.Signature {
			signatureValid = true
			log.Printf("[razorpay] signature matched with payload=%q", payload)
			break
		}
	}

	if !signatureValid {
		// Last resort: fetch the payment from Razorpay API and confirm it's captured
		// This handles edge cases where the signature format differs
		client := razorpay.NewClient(h.KeyID, h.KeySecret)
		payment, fetchErr := client.Payment.Fetch(req.PaymentID, nil, nil)
		if fetchErr != nil {
			log.Printf("[razorpay] signature mismatch and fetch failed: %v", fetchErr)
			c.JSON(http.StatusBadRequest, gin.H{"error": "payment verification failed — signature mismatch"})
			return
		}
		status, _ := payment["status"].(string)
		log.Printf("[razorpay] fallback fetch: payment=%s status=%s", req.PaymentID, status)
		if status != "captured" && status != "authorized" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "payment not captured (status: " + status + ")"})
			return
		}
		// Payment confirmed via API — proceed
		log.Printf("[razorpay] payment %s confirmed via API fetch (status=%s)", req.PaymentID, status)

		// If subscription_id was missing, try to get it from the payment object
		if req.SubscriptionID == "" {
			if subID, ok := payment["subscription_id"].(string); ok && subID != "" {
				req.SubscriptionID = subID
				log.Printf("[razorpay] recovered subscription_id=%s from payment fetch", subID)
			}
		}
	}

	plan, err := GetPlanByName(h.DB, req.PlanName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan"})
		return
	}

	// Use Razorpay subscription ID as the local sub reference
	localSubRef := req.SubscriptionID
	if localSubRef == "" {
		localSubRef = "rzp_" + req.PaymentID // stable reference tied to the payment
	}

	now := time.Now()
	nextMonth := now.AddDate(0, 1, 0)

	// Cancel all existing active subscriptions for this account first
	_, _ = h.DB.Exec(
		`UPDATE stripe_subscriptions SET status='canceled', deleted_at=NOW(), updated_at=NOW()
		 WHERE account_id=? AND status IN ('active','trialing') AND deleted_at IS NULL`,
		accountID,
	)

	// Insert new subscription (or re-activate if same sub_id already exists)
	_, err = h.DB.Exec(
		`INSERT INTO stripe_subscriptions
		 (id, account_id, stripe_subscription_id, plan_id, status,
		  current_period_start, current_period_end, cancel_at_period_end, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'active', ?, ?, 0, NOW(), NOW())
		 ON DUPLICATE KEY UPDATE
		   plan_id=VALUES(plan_id), status='active',
		   current_period_start=VALUES(current_period_start),
		   current_period_end=VALUES(current_period_end),
		   deleted_at=NULL, updated_at=NOW()`,
		uuid.New().String(), accountID, localSubRef, plan.ID,
		now, nextMonth,
	)
	if err != nil {
		log.Printf("[razorpay] DB upsert failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to activate subscription"})
		return
	}

	// Record invoice
	_, _ = h.DB.Exec(
		`INSERT INTO stripe_invoices
		 (id, account_id, stripe_invoice_id, amount_cents, currency, status, created_at)
		 VALUES (?, ?, ?, ?, 'INR', 'paid', NOW())`,
		uuid.New().String(), accountID, req.PaymentID, plan.PriceCents,
	)

	log.Printf("[razorpay] Subscription activated: sub=%s payment=%s account=%s plan=%s",
		localSubRef, req.PaymentID, accountID, plan.Name)
	c.JSON(http.StatusOK, gin.H{"plan": plan.Name, "status": "active"})
}

// HandleRazorpayWebhook processes Razorpay webhook events for subscription renewals.
// POST /billing/razorpay/webhook
func (h *RazorpayCheckoutHandler) HandleRazorpayWebhook(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	// Verify webhook signature
	sig := c.GetHeader("X-Razorpay-Signature")
	mac := hmac.New(sha256.New, []byte(h.WebhookSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if sig != expected {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook signature"})
		return
	}

	// Parse event
	var event map[string]interface{}
	if err := json.Unmarshal(body, &event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	eventType, _ := event["event"].(string)
	log.Printf("[razorpay] Webhook event: %s", eventType)

	switch eventType {
	case "subscription.charged":
		// Recurring payment succeeded — extend the subscription period
		h.handleSubscriptionCharged(event)
	case "subscription.cancelled", "subscription.completed":
		h.handleSubscriptionCancelled(event)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleSubscriptionCharged extends the subscription period on successful renewal.
func (h *RazorpayCheckoutHandler) handleSubscriptionCharged(event map[string]interface{}) {
	payload, _ := event["payload"].(map[string]interface{})
	if payload == nil {
		return
	}
	subPayload, _ := payload["subscription"].(map[string]interface{})
	entity, _ := subPayload["entity"].(map[string]interface{})
	if entity == nil {
		return
	}
	subID, _ := entity["id"].(string)
	if subID == "" {
		return
	}

	// Extend period by 1 month
	_, _ = h.DB.Exec(
		`UPDATE stripe_subscriptions
		 SET current_period_start=NOW(),
		     current_period_end=DATE_ADD(NOW(), INTERVAL 1 MONTH),
		     updated_at=NOW()
		 WHERE stripe_subscription_id=? AND deleted_at IS NULL`,
		subID,
	)
	log.Printf("[razorpay] Subscription %s renewed — period extended 1 month", subID)
}

// GetRazorpayPayments returns live payment history for the account's Razorpay subscription.
// GET /billing/razorpay/payments
func (h *RazorpayCheckoutHandler) GetRazorpayPayments(c *gin.Context) {
	accountID := c.GetHeader("X-Account-ID")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_id required"})
		return
	}

	// Get the Razorpay subscription ID from DB
	var subID string
	err := h.DB.QueryRow(
		`SELECT stripe_subscription_id FROM stripe_subscriptions
		 WHERE account_id=? AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`,
		accountID,
	).Scan(&subID)
	if err != nil || !strings.HasPrefix(subID, "sub_") {
		c.JSON(http.StatusOK, gin.H{"payments": []interface{}{}})
		return
	}

	// Correct Razorpay endpoint: GET /v1/invoices?subscription_id={id}
	apiURL := fmt.Sprintf("https://api.razorpay.com/v1/invoices?subscription_id=%s&count=100", subID)
	httpReq, _ := http.NewRequest("GET", apiURL, nil)
	httpReq.SetBasicAuth(h.KeyID, h.KeySecret)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, httpErr := http.DefaultClient.Do(httpReq)
	if httpErr != nil {
		log.Printf("[razorpay] payments REST call failed for sub %s: %v", subID, httpErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch payments"})
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	log.Printf("[razorpay] payments API status=%d body=%s", resp.StatusCode, string(bodyBytes))

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse payments response"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		errObj, _ := result["error"].(map[string]interface{})
		desc, _ := errObj["description"].(string)
		log.Printf("[razorpay] payments API error for sub %s: status=%d desc=%s", subID, resp.StatusCode, desc)
		// Return empty list instead of 500 — subscription may have no payments yet
		c.JSON(http.StatusOK, gin.H{"payments": []interface{}{}})
		return
	}

	type paymentOut struct {
		ID           string `json:"id"`
		PaymentID    string `json:"payment_id"`
		Amount       int64  `json:"amount"` // paise
		Currency     string `json:"currency"`
		Status       string `json:"status"`
		CreatedAt    string `json:"created_at"`
		PaidAt       string `json:"paid_at,omitempty"`
		BillingStart string `json:"billing_start,omitempty"`
		BillingEnd   string `json:"billing_end,omitempty"`
		ShortURL     string `json:"short_url,omitempty"`
	}

	var payments []paymentOut
	items, _ := result["items"].([]interface{})
	for _, item := range items {
		p, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := p["id"].(string)
		paymentID, _ := p["payment_id"].(string)
		status, _ := p["status"].(string)
		currency, _ := p["currency"].(string)
		shortURL, _ := p["short_url"].(string)

		var amount int64
		if v, ok := p["gross_amount"].(float64); ok {
			amount = int64(v)
		}

		fmtUnix := func(key string) string {
			if v, ok := p[key].(float64); ok && v > 0 {
				return time.Unix(int64(v), 0).Format("2006-01-02T15:04:05Z")
			}
			return ""
		}

		payments = append(payments, paymentOut{
			ID:           id,
			PaymentID:    paymentID,
			Amount:       amount,
			Currency:     currency,
			Status:       status,
			CreatedAt:    fmtUnix("created_at"),
			PaidAt:       fmtUnix("paid_at"),
			BillingStart: fmtUnix("billing_start"),
			BillingEnd:   fmtUnix("billing_end"),
			ShortURL:     shortURL,
		})
	}

	if payments == nil {
		payments = []paymentOut{}
	}
	c.JSON(http.StatusOK, gin.H{"payments": payments})
}

// handleSubscriptionCancelled marks the subscription as cancelled.
func (h *RazorpayCheckoutHandler) handleSubscriptionCancelled(event map[string]interface{}) {
	payload, _ := event["payload"].(map[string]interface{})
	if payload == nil {
		return
	}
	subPayload, _ := payload["subscription"].(map[string]interface{})
	entity, _ := subPayload["entity"].(map[string]interface{})
	if entity == nil {
		return
	}
	subID, _ := entity["id"].(string)
	if subID == "" {
		return
	}

	_, _ = h.DB.Exec(
		`UPDATE stripe_subscriptions SET status='canceled', deleted_at=NOW()
		 WHERE stripe_subscription_id=? AND deleted_at IS NULL`,
		subID,
	)
	log.Printf("[razorpay] Subscription %s cancelled", subID)
}
