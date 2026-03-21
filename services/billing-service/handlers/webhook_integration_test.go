package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

const testWebhookSecret = "whsec_test_secret_key_for_testing"

// newWebhookRouter wires up the WebhookHandler for testing.
func newWebhookRouter(h *WebhookHandler) *gin.Engine {
	r := gin.New()
	r.POST("/billing/webhook", h.HandleWebhook)
	return r
}

// generateStripeSignature creates a valid Stripe-Signature header value.
// Stripe uses: t=<unix_timestamp>,v1=<hmac_sha256(secret, t.payload)>
func generateStripeSignature(secret string, payload []byte, t int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d", t)))
	mac.Write([]byte("."))
	mac.Write(payload)
	sig := fmt.Sprintf("%x", mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", t, sig)
}

// buildStripeEvent constructs a minimal Stripe event JSON payload.
// api_version must match what stripe-go v76 expects (2023-10-16).
func buildStripeEvent(eventID, eventType string, dataRaw json.RawMessage) []byte {
	event := map[string]any{
		"id":          eventID,
		"type":        eventType,
		"api_version": "2023-10-16",
		"data": map[string]any{
			"object": json.RawMessage(dataRaw),
		},
	}
	b, _ := json.Marshal(event)
	return b
}

// postWebhook sends a POST to /billing/webhook with the given body and Stripe-Signature header.
func postWebhook(r *gin.Engine, body []byte, sigHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/billing/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if sigHeader != "" {
		req.Header.Set("Stripe-Signature", sigHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ─── Signature Verification ──────────────────────────────────────────────────

// TestWebhook_InvalidSignature_Returns400 verifies that a request with a bad
// Stripe-Signature header is rejected with HTTP 400.
// Validates: Requirements 26.1, 26.2
func TestWebhook_InvalidSignature_Returns400(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	// Audit log for failed signature
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &WebhookHandler{DB: db, WebhookSecret: testWebhookSecret}
	r := newWebhookRouter(h)

	payload := []byte(`{"id":"evt_bad","type":"invoice.payment_succeeded","data":{"object":{}}}`)
	w := postWebhook(r, payload, "t=12345,v1=invalidsignature")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["error"] == "" {
		t.Error("expected non-empty error message")
	}
}

// TestWebhook_MissingSignature_Returns400 verifies that a request with no
// Stripe-Signature header is rejected with HTTP 400.
// Validates: Requirements 26.1, 26.2
func TestWebhook_MissingSignature_Returns400(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	// Audit log for failed signature
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &WebhookHandler{DB: db, WebhookSecret: testWebhookSecret}
	r := newWebhookRouter(h)

	payload := []byte(`{"id":"evt_nosig","type":"invoice.payment_succeeded","data":{"object":{}}}`)
	w := postWebhook(r, payload, "") // no signature header

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── Idempotency ─────────────────────────────────────────────────────────────

// TestWebhook_DuplicateEventID_Returns200WithoutReprocessing verifies that
// sending the same event ID twice results in HTTP 200 on the second call
// without re-processing the event.
// Validates: Requirements 26.4
func TestWebhook_DuplicateEventID_Returns200WithoutReprocessing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	eventID := "evt_duplicate_001"
	ts := time.Now().Unix()

	subData, _ := json.Marshal(map[string]any{
		"id":                   "sub_dup",
		"status":               "active",
		"cancel_at_period_end": false,
		"current_period_start": ts,
		"current_period_end":   ts + 2592000,
		"customer":             map[string]any{"id": "cus_dup"},
		"items": map[string]any{
			"data": []any{
				map[string]any{"price": map[string]any{"id": "price_base"}},
			},
		},
	})

	payload := buildStripeEvent(eventID, "customer.subscription.updated", subData)
	sig := generateStripeSignature(testWebhookSecret, payload, ts)

	// First call: event not yet processed
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_logs`).
		WithArgs("webhook_processed", eventID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// processEvent → handleSubscriptionUpdated → DB update
	mock.ExpectExec(`UPDATE stripe_subscriptions`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mark event as processed
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &WebhookHandler{DB: db, WebhookSecret: testWebhookSecret}
	r := newWebhookRouter(h)

	w1 := postWebhook(r, payload, sig)
	if w1.Code != http.StatusOK {
		t.Fatalf("first call: expected 200, got %d: %s", w1.Code, w1.Body.String())
	}

	// Second call: event already processed — only the idempotency check query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_logs`).
		WithArgs("webhook_processed", eventID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	w2 := postWebhook(r, payload, sig)
	if w2.Code != http.StatusOK {
		t.Fatalf("second call: expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "already processed" {
		t.Errorf("expected status='already processed', got %v", resp["status"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet DB expectations: %v", err)
	}
}

// ─── Event Type Processing ────────────────────────────────────────────────────

// TestWebhook_SubscriptionCreated_UpsertsToDB verifies that a valid
// customer.subscription.created event upserts a subscription record.
// Validates: Requirements 26.3
func TestWebhook_SubscriptionCreated_UpsertsToDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	eventID := "evt_sub_created_001"
	ts := time.Now().Unix()

	subData, _ := json.Marshal(map[string]any{
		"id":                   "sub_new_001",
		"status":               "active",
		"cancel_at_period_end": false,
		"current_period_start": ts,
		"current_period_end":   ts + 2592000,
		"customer":             map[string]any{"id": "cus_001"},
		"items": map[string]any{
			"data": []any{
				map[string]any{"price": map[string]any{"id": "price_pro"}},
			},
		},
	})

	payload := buildStripeEvent(eventID, "customer.subscription.created", subData)
	sig := generateStripeSignature(testWebhookSecret, payload, ts)

	// Idempotency check: not yet processed
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_logs`).
		WithArgs("webhook_processed", eventID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// getAccountIDByStripeCustomer
	mock.ExpectQuery(`SELECT account_id FROM stripe_customers`).
		WithArgs("cus_001").
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow("acct-001"))

	// getPlanIDByStripePrice
	mock.ExpectQuery(`SELECT id FROM subscription_plans`).
		WithArgs("price_pro").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("plan-pro-id"))

	// INSERT INTO stripe_subscriptions ... ON DUPLICATE KEY UPDATE
	mock.ExpectExec(`INSERT INTO stripe_subscriptions`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mark event as processed
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &WebhookHandler{DB: db, WebhookSecret: testWebhookSecret}
	r := newWebhookRouter(h)

	w := postWebhook(r, payload, sig)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", resp["status"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet DB expectations: %v", err)
	}
}

// TestWebhook_SubscriptionUpdated_UpdatesDB verifies that a valid
// customer.subscription.updated event updates the subscription record.
// Validates: Requirements 26.3
func TestWebhook_SubscriptionUpdated_UpdatesDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	eventID := "evt_sub_updated_001"
	ts := time.Now().Unix()

	subData, _ := json.Marshal(map[string]any{
		"id":                   "sub_upd_001",
		"status":               "active",
		"cancel_at_period_end": false,
		"current_period_start": ts,
		"current_period_end":   ts + 2592000,
		"customer":             map[string]any{"id": "cus_002"},
		"items": map[string]any{
			"data": []any{
				map[string]any{"price": map[string]any{"id": "price_enterprise"}},
			},
		},
	})

	payload := buildStripeEvent(eventID, "customer.subscription.updated", subData)
	sig := generateStripeSignature(testWebhookSecret, payload, ts)

	// Idempotency check
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_logs`).
		WithArgs("webhook_processed", eventID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// getPlanIDByStripePrice
	mock.ExpectQuery(`SELECT id FROM subscription_plans`).
		WithArgs("price_enterprise").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("plan-ent-id"))

	// UPDATE stripe_subscriptions
	mock.ExpectExec(`UPDATE stripe_subscriptions`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mark event as processed
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &WebhookHandler{DB: db, WebhookSecret: testWebhookSecret}
	r := newWebhookRouter(h)

	w := postWebhook(r, payload, sig)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet DB expectations: %v", err)
	}
}

// TestWebhook_SubscriptionDeleted_CancelsSubscription verifies that a valid
// customer.subscription.deleted event marks the subscription as canceled.
// Validates: Requirements 26.3
func TestWebhook_SubscriptionDeleted_CancelsSubscription(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	eventID := "evt_sub_deleted_001"
	ts := time.Now().Unix()

	subData, _ := json.Marshal(map[string]any{
		"id":                   "sub_del_001",
		"status":               "canceled",
		"cancel_at_period_end": false,
		"current_period_start": ts - 2592000,
		"current_period_end":   ts,
		"customer":             map[string]any{"id": "cus_003"},
		"items": map[string]any{
			"data": []any{
				map[string]any{"price": map[string]any{"id": "price_base"}},
			},
		},
	})

	payload := buildStripeEvent(eventID, "customer.subscription.deleted", subData)
	sig := generateStripeSignature(testWebhookSecret, payload, ts)

	// Idempotency check
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_logs`).
		WithArgs("webhook_processed", eventID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// UPDATE stripe_subscriptions SET status='canceled'
	mock.ExpectExec(`UPDATE stripe_subscriptions SET status`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mark event as processed
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &WebhookHandler{DB: db, WebhookSecret: testWebhookSecret}
	r := newWebhookRouter(h)

	w := postWebhook(r, payload, sig)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet DB expectations: %v", err)
	}
}

// TestWebhook_InvoicePaymentSucceeded_UpsertsInvoice verifies that a valid
// invoice.payment_succeeded event upserts an invoice record.
// Validates: Requirements 26.3
func TestWebhook_InvoicePaymentSucceeded_UpsertsInvoice(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	eventID := "evt_inv_paid_001"
	ts := time.Now().Unix()

	invData, _ := json.Marshal(map[string]any{
		"id":          "in_paid_001",
		"status":      "paid",
		"amount_due":  2000,
		"currency":    "usd",
		"invoice_pdf": "https://stripe.com/invoice.pdf",
		"customer":    map[string]any{"id": "cus_004", "email": "owner@example.com"},
	})

	payload := buildStripeEvent(eventID, "invoice.payment_succeeded", invData)
	sig := generateStripeSignature(testWebhookSecret, payload, ts)

	// Idempotency check
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_logs`).
		WithArgs("webhook_processed", eventID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// getAccountIDByStripeCustomer
	mock.ExpectQuery(`SELECT account_id FROM stripe_customers`).
		WithArgs("cus_004").
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow("acct-004"))

	// upsertInvoice → INSERT INTO stripe_invoices
	mock.ExpectExec(`INSERT INTO stripe_invoices`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mark event as processed
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &WebhookHandler{DB: db, WebhookSecret: testWebhookSecret}
	r := newWebhookRouter(h)

	w := postWebhook(r, payload, sig)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet DB expectations: %v", err)
	}
}

// TestWebhook_InvoicePaymentFailed_UpsertsInvoiceAndNotifies verifies that a
// valid invoice.payment_failed event upserts the invoice and triggers
// notification logic (audit log for payment_failed_notification).
// Validates: Requirements 26.3, 3.8
func TestWebhook_InvoicePaymentFailed_UpsertsInvoiceAndNotifies(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	eventID := "evt_inv_failed_001"
	ts := time.Now().Unix()

	invData, _ := json.Marshal(map[string]any{
		"id":         "in_fail_001",
		"status":     "open",
		"amount_due": 2000,
		"currency":   "usd",
		"customer":   map[string]any{"id": "cus_005", "email": "owner@example.com"},
	})

	payload := buildStripeEvent(eventID, "invoice.payment_failed", invData)
	sig := generateStripeSignature(testWebhookSecret, payload, ts)

	// Idempotency check
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_logs`).
		WithArgs("webhook_processed", eventID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// getAccountIDByStripeCustomer
	mock.ExpectQuery(`SELECT account_id FROM stripe_customers`).
		WithArgs("cus_005").
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow("acct-005"))

	// upsertInvoice → INSERT INTO stripe_invoices
	mock.ExpectExec(`INSERT INTO stripe_invoices`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// sendPaymentFailureNotifications → get owner email
	mock.ExpectQuery(`SELECT u.email FROM users`).
		WithArgs("acct-005").
		WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow("owner@example.com"))

	// sendPaymentFailureNotifications → audit log
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mark event as processed
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// EmailCfg has no SMTP host so sendEmail will fail silently — that's fine
	h := &WebhookHandler{
		DB:            db,
		WebhookSecret: testWebhookSecret,
		EmailCfg:      EmailConfig{},
	}
	r := newWebhookRouter(h)

	w := postWebhook(r, payload, sig)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet DB expectations: %v", err)
	}
}
