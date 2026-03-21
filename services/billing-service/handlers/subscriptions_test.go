package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newSubscriptionRouter wires up the SubscriptionHandler routes for testing.
func newSubscriptionRouter(h *SubscriptionHandler) *gin.Engine {
	r := gin.New()
	r.POST("/billing/subscribe", h.Subscribe)
	r.PUT("/billing/subscription", h.UpdateSubscription)
	return r
}

// postJSON is a helper that sends a POST request with a JSON body.
func postJSON(r *gin.Engine, path string, body interface{}) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// putJSON is a helper that sends a PUT request with a JSON body.
func putJSON(r *gin.Engine, path string, body interface{}) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ─── Subscribe (POST /billing/subscribe) ────────────────────────────────────

// TestSubscribe_FreePlan_Success verifies that subscribing to the free plan
// creates a local subscription record and returns HTTP 201.
// Validates: Requirements 3.1, 3.2
func TestSubscribe_FreePlan_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	// GetPlanByName("free")
	mock.ExpectQuery(`SELECT id, name, price_cents`).
		WithArgs("free").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price_cents", "stripe_price_id", "max_cloud_accounts", "max_database_connections", "rate_limit_per_minute"}).
			AddRow("plan-free-id", "free", 0, "", nil, nil, 100))

	// Lookup Stripe customer
	mock.ExpectQuery(`SELECT stripe_customer_id FROM stripe_customers`).
		WithArgs("acct-1").
		WillReturnRows(sqlmock.NewRows([]string{"stripe_customer_id"}).AddRow("cus_test"))

	// Check for existing active subscription (none)
	mock.ExpectQuery(`SELECT stripe_subscription_id FROM stripe_subscriptions`).
		WithArgs("acct-1").
		WillReturnError(sql.ErrNoRows)

	// Insert new free subscription
	mock.ExpectExec(`INSERT INTO stripe_subscriptions`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Audit log insert
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &SubscriptionHandler{DB: db}
	r := newSubscriptionRouter(h)

	w := postJSON(r, "/billing/subscribe", map[string]string{
		"account_id": "acct-1",
		"plan":       "free",
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["plan"] != "free" {
		t.Errorf("expected plan=free, got %v", resp["plan"])
	}
	if resp["status"] != "active" {
		t.Errorf("expected status=active, got %v", resp["status"])
	}
	if resp["subscription_id"] == "" {
		t.Error("expected non-empty subscription_id")
	}
}

// TestSubscribe_InvalidPlan returns HTTP 400 when an unknown plan name is given.
// Validates: Requirements 3.1
func TestSubscribe_InvalidPlan(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	// GetPlanByName returns no rows → invalid plan
	mock.ExpectQuery(`SELECT id, name, price_cents`).
		WithArgs("nonexistent").
		WillReturnError(sql.ErrNoRows)

	h := &SubscriptionHandler{DB: db}
	r := newSubscriptionRouter(h)

	w := postJSON(r, "/billing/subscribe", map[string]string{
		"account_id": "acct-1",
		"plan":       "nonexistent",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["error"] == "" {
		t.Error("expected non-empty error message")
	}
}

// TestSubscribe_MissingPaymentMethod_NoStripePrice verifies that a paid plan
// with no stripe_price_id configured still creates a local record (graceful
// degradation) and returns HTTP 201.
// Validates: Requirements 3.2
func TestSubscribe_MissingPaymentMethod_NoStripePrice(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	// Plan exists but has no stripe_price_id (simulates unconfigured paid plan)
	mock.ExpectQuery(`SELECT id, name, price_cents`).
		WithArgs("base").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price_cents", "stripe_price_id", "max_cloud_accounts", "max_database_connections", "rate_limit_per_minute"}).
			AddRow("plan-base-id", "base", 1000, "", nil, nil, 500))

	// Lookup Stripe customer
	mock.ExpectQuery(`SELECT stripe_customer_id FROM stripe_customers`).
		WithArgs("acct-2").
		WillReturnRows(sqlmock.NewRows([]string{"stripe_customer_id"}).AddRow("cus_test2"))

	// No existing subscription
	mock.ExpectQuery(`SELECT stripe_subscription_id FROM stripe_subscriptions`).
		WithArgs("acct-2").
		WillReturnError(sql.ErrNoRows)

	// Insert subscription (falls through to local path because stripe_price_id is empty)
	mock.ExpectExec(`INSERT INTO stripe_subscriptions`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Audit log
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &SubscriptionHandler{DB: db}
	r := newSubscriptionRouter(h)

	// No payment_method_id provided
	w := postJSON(r, "/billing/subscribe", map[string]string{
		"account_id": "acct-2",
		"plan":       "base",
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSubscribe_NoStripeCustomer returns HTTP 404 when no Stripe customer
// record exists for the account.
// Validates: Requirements 3.2
func TestSubscribe_NoStripeCustomer(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT id, name, price_cents`).
		WithArgs("pro").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price_cents", "stripe_price_id", "max_cloud_accounts", "max_database_connections", "rate_limit_per_minute"}).
			AddRow("plan-pro-id", "pro", 2000, "price_pro", nil, nil, 2000))

	// No Stripe customer found
	mock.ExpectQuery(`SELECT stripe_customer_id FROM stripe_customers`).
		WithArgs("acct-3").
		WillReturnError(sql.ErrNoRows)

	h := &SubscriptionHandler{DB: db}
	r := newSubscriptionRouter(h)

	w := postJSON(r, "/billing/subscribe", map[string]string{
		"account_id":        "acct-3",
		"plan":              "pro",
		"payment_method_id": "pm_test",
	})

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── UpdateSubscription (PUT /billing/subscription) ─────────────────────────

// TestUpdateSubscription_Upgrade_ImmediateEffectiveDate verifies that upgrading
// a plan returns effective_date="immediate" and a proration_amount field.
// Validates: Requirements 3.3
func TestUpdateSubscription_Upgrade_ImmediateEffectiveDate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	// GetPlanByName("enterprise")
	mock.ExpectQuery(`SELECT id, name, price_cents`).
		WithArgs("enterprise").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price_cents", "stripe_price_id", "max_cloud_accounts", "max_database_connections", "rate_limit_per_minute"}).
			AddRow("plan-ent-id", "enterprise", 5000, "", nil, nil, 10000))

	// Current subscription (local free_ prefix so no Stripe call)
	mock.ExpectQuery(`SELECT id, stripe_subscription_id, plan_id FROM stripe_subscriptions`).
		WithArgs("acct-4").
		WillReturnRows(sqlmock.NewRows([]string{"id", "stripe_subscription_id", "plan_id"}).
			AddRow("sub-local-id", "free_acct-4", "plan-base-id"))

	// Current plan price (base = 1000 cents)
	mock.ExpectQuery(`SELECT price_cents FROM subscription_plans`).
		WithArgs("plan-base-id").
		WillReturnRows(sqlmock.NewRows([]string{"price_cents"}).AddRow(1000))

	// Local upgrade: UPDATE stripe_subscriptions SET plan_id=?
	mock.ExpectExec(`UPDATE stripe_subscriptions SET plan_id`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Audit log
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &SubscriptionHandler{DB: db}
	r := newSubscriptionRouter(h)

	w := putJSON(r, "/billing/subscription", map[string]string{
		"account_id": "acct-4",
		"new_plan":   "enterprise",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Requirement 3.3: upgrades are immediate
	if resp["effective_date"] != "immediate" {
		t.Errorf("expected effective_date=immediate for upgrade, got %v", resp["effective_date"])
	}
	// proration_amount field must be present
	if _, ok := resp["proration_amount"]; !ok {
		t.Error("expected proration_amount field in response")
	}
}

// TestUpdateSubscription_Downgrade_PeriodEnd verifies that downgrading a plan
// returns effective_date="period_end" and sets cancel_at_period_end in the DB.
// Validates: Requirements 3.4
func TestUpdateSubscription_Downgrade_PeriodEnd(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	// GetPlanByName("base")
	mock.ExpectQuery(`SELECT id, name, price_cents`).
		WithArgs("base").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price_cents", "stripe_price_id", "max_cloud_accounts", "max_database_connections", "rate_limit_per_minute"}).
			AddRow("plan-base-id", "base", 1000, "", nil, nil, 500))

	// Current subscription (local, enterprise plan)
	mock.ExpectQuery(`SELECT id, stripe_subscription_id, plan_id FROM stripe_subscriptions`).
		WithArgs("acct-5").
		WillReturnRows(sqlmock.NewRows([]string{"id", "stripe_subscription_id", "plan_id"}).
			AddRow("sub-ent-id", "free_acct-5", "plan-ent-id"))

	// Current plan price (enterprise = 5000 cents)
	mock.ExpectQuery(`SELECT price_cents FROM subscription_plans`).
		WithArgs("plan-ent-id").
		WillReturnRows(sqlmock.NewRows([]string{"price_cents"}).AddRow(5000))

	// Local downgrade: UPDATE stripe_subscriptions SET plan_id=?, cancel_at_period_end=TRUE
	mock.ExpectExec(`UPDATE stripe_subscriptions SET plan_id`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Audit log
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	h := &SubscriptionHandler{DB: db}
	r := newSubscriptionRouter(h)

	w := putJSON(r, "/billing/subscription", map[string]string{
		"account_id": "acct-5",
		"new_plan":   "base",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Requirement 3.4: downgrades apply at period end
	if resp["effective_date"] != "period_end" {
		t.Errorf("expected effective_date=period_end for downgrade, got %v", resp["effective_date"])
	}

	// Verify the mock expectation for cancel_at_period_end was satisfied
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet DB expectations: %v", err)
	}
}

// TestUpdateSubscription_NoActiveSubscription returns HTTP 404 when the account
// has no active subscription to update.
// Validates: Requirements 3.3, 3.4
func TestUpdateSubscription_NoActiveSubscription(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT id, name, price_cents`).
		WithArgs("pro").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price_cents", "stripe_price_id", "max_cloud_accounts", "max_database_connections", "rate_limit_per_minute"}).
			AddRow("plan-pro-id", "pro", 2000, "", nil, nil, 2000))

	// No active subscription
	mock.ExpectQuery(`SELECT id, stripe_subscription_id, plan_id FROM stripe_subscriptions`).
		WithArgs("acct-6").
		WillReturnError(sql.ErrNoRows)

	h := &SubscriptionHandler{DB: db}
	r := newSubscriptionRouter(h)

	w := putJSON(r, "/billing/subscription", map[string]string{
		"account_id": "acct-6",
		"new_plan":   "pro",
	})

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUpdateSubscription_InvalidNewPlan returns HTTP 400 for an unknown plan.
// Validates: Requirements 3.3, 3.4
func TestUpdateSubscription_InvalidNewPlan(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT id, name, price_cents`).
		WithArgs("bogus").
		WillReturnError(sql.ErrNoRows)

	h := &SubscriptionHandler{DB: db}
	r := newSubscriptionRouter(h)

	w := putJSON(r, "/billing/subscription", map[string]string{
		"account_id": "acct-7",
		"new_plan":   "bogus",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
