package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

// newPlanLimitsRouter wires up the PlanLimitsHandler routes for testing.
func newPlanLimitsRouter(h *PlanLimitsHandler) *gin.Engine {
	r := gin.New()
	r.GET("/billing/plan-limits", func(c *gin.Context) {
		// Inject account_id from query param for test convenience
		if id := c.Query("account_id"); id != "" {
			c.Set("account_id", id)
		}
		h.GetPlanLimits(c)
	})
	return r
}

// getPlanLimits is a helper that sends a GET request with account_id as a query param.
func getPlanLimits(r *gin.Engine, accountID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/billing/plan-limits?account_id="+accountID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ─── buildPlanLimitsResponse unit tests ─────────────────────────────────────

// TestBuildPlanLimitsResponse_FreePlan verifies Free plan limits:
// max 1 cloud account, max 2 database connections.
// Validates: Requirements 12.1, 12.5
func TestBuildPlanLimitsResponse_FreePlan(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	maxCA := 1
	maxDB := 2

	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("acct-free").
		WillReturnRows(sqlmock.NewRows([]string{"name", "max_cloud_accounts", "max_database_connections"}).
			AddRow("free", maxCA, maxDB))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("acct-free").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("acct-free").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	resp, err := buildPlanLimitsResponse(db, "acct-free")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.PlanName != "free" {
		t.Errorf("expected plan_name=free, got %s", resp.PlanName)
	}
	if resp.MaxCloudAccounts == nil || *resp.MaxCloudAccounts != 1 {
		t.Errorf("expected max_cloud_accounts=1, got %v", resp.MaxCloudAccounts)
	}
	if resp.MaxDatabaseConnections == nil || *resp.MaxDatabaseConnections != 2 {
		t.Errorf("expected max_database_connections=2, got %v", resp.MaxDatabaseConnections)
	}
}

// TestBuildPlanLimitsResponse_BasePlan verifies Base plan limits:
// max 3 cloud accounts, max 5 database connections.
// Validates: Requirements 12.2, 12.6
func TestBuildPlanLimitsResponse_BasePlan(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	maxCA := 3
	maxDB := 5

	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("acct-base").
		WillReturnRows(sqlmock.NewRows([]string{"name", "max_cloud_accounts", "max_database_connections"}).
			AddRow("base", maxCA, maxDB))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("acct-base").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("acct-base").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	resp, err := buildPlanLimitsResponse(db, "acct-base")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.PlanName != "base" {
		t.Errorf("expected plan_name=base, got %s", resp.PlanName)
	}
	if resp.MaxCloudAccounts == nil || *resp.MaxCloudAccounts != 3 {
		t.Errorf("expected max_cloud_accounts=3, got %v", resp.MaxCloudAccounts)
	}
	if resp.MaxDatabaseConnections == nil || *resp.MaxDatabaseConnections != 5 {
		t.Errorf("expected max_database_connections=5, got %v", resp.MaxDatabaseConnections)
	}
}

// TestBuildPlanLimitsResponse_ProPlan verifies Pro plan limits:
// max 10 cloud accounts, unlimited database connections (nil).
// Validates: Requirements 12.3, 12.7
func TestBuildPlanLimitsResponse_ProPlan(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	maxCA := 10

	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("acct-pro").
		WillReturnRows(sqlmock.NewRows([]string{"name", "max_cloud_accounts", "max_database_connections"}).
			AddRow("pro", maxCA, nil)) // nil = unlimited databases

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("acct-pro").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("acct-pro").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	resp, err := buildPlanLimitsResponse(db, "acct-pro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.PlanName != "pro" {
		t.Errorf("expected plan_name=pro, got %s", resp.PlanName)
	}
	if resp.MaxCloudAccounts == nil || *resp.MaxCloudAccounts != 10 {
		t.Errorf("expected max_cloud_accounts=10, got %v", resp.MaxCloudAccounts)
	}
	if resp.MaxDatabaseConnections != nil {
		t.Errorf("expected max_database_connections=nil (unlimited) for pro, got %v", *resp.MaxDatabaseConnections)
	}
	// Unlimited databases means DatabaseConnectionsRemaining should also be nil
	if resp.DatabaseConnectionsRemaining != nil {
		t.Errorf("expected database_connections_remaining=nil for unlimited plan, got %v", *resp.DatabaseConnectionsRemaining)
	}
}

// TestBuildPlanLimitsResponse_EnterprisePlan verifies Enterprise plan limits:
// unlimited cloud accounts and unlimited database connections (both nil).
// Validates: Requirements 12.4, 12.7
func TestBuildPlanLimitsResponse_EnterprisePlan(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("acct-ent").
		WillReturnRows(sqlmock.NewRows([]string{"name", "max_cloud_accounts", "max_database_connections"}).
			AddRow("enterprise", nil, nil)) // both nil = unlimited

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("acct-ent").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(50))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("acct-ent").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))

	resp, err := buildPlanLimitsResponse(db, "acct-ent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.PlanName != "enterprise" {
		t.Errorf("expected plan_name=enterprise, got %s", resp.PlanName)
	}
	if resp.MaxCloudAccounts != nil {
		t.Errorf("expected max_cloud_accounts=nil (unlimited) for enterprise, got %v", *resp.MaxCloudAccounts)
	}
	if resp.MaxDatabaseConnections != nil {
		t.Errorf("expected max_database_connections=nil (unlimited) for enterprise, got %v", *resp.MaxDatabaseConnections)
	}
	if resp.CloudAccountsRemaining != nil {
		t.Errorf("expected cloud_accounts_remaining=nil for unlimited plan, got %v", *resp.CloudAccountsRemaining)
	}
	if resp.DatabaseConnectionsRemaining != nil {
		t.Errorf("expected database_connections_remaining=nil for unlimited plan, got %v", *resp.DatabaseConnectionsRemaining)
	}
}

// TestBuildPlanLimitsResponse_NoSubscription_DefaultsToFree verifies that when
// no active subscription exists, the response defaults to Free plan limits.
// Validates: Requirements 12.1, 12.5
func TestBuildPlanLimitsResponse_NoSubscription_DefaultsToFree(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("acct-nosub").
		WillReturnError(sqlmock.ErrCancelled) // simulate sql.ErrNoRows via no-rows path

	// Re-create with proper ErrNoRows simulation
	db2, mock2, _ := sqlmock.New()
	defer db2.Close()

	mock2.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("acct-nosub").
		WillReturnRows(sqlmock.NewRows([]string{"name", "max_cloud_accounts", "max_database_connections"})) // empty rows → ErrNoRows

	mock2.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("acct-nosub").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock2.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("acct-nosub").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	resp, err := buildPlanLimitsResponse(db2, "acct-nosub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.PlanName != "free" {
		t.Errorf("expected default plan_name=free, got %s", resp.PlanName)
	}
	if resp.MaxCloudAccounts == nil || *resp.MaxCloudAccounts != 1 {
		t.Errorf("expected default max_cloud_accounts=1, got %v", resp.MaxCloudAccounts)
	}
	if resp.MaxDatabaseConnections == nil || *resp.MaxDatabaseConnections != 2 {
		t.Errorf("expected default max_database_connections=2, got %v", resp.MaxDatabaseConnections)
	}
}

// ─── Remaining capacity calculation tests ───────────────────────────────────

// TestBuildPlanLimitsResponse_RemainingCapacity_UnderLimit verifies that
// remaining capacity is correctly calculated when usage is under the limit.
// Validates: Requirements 12.1, 12.5
func TestBuildPlanLimitsResponse_RemainingCapacity_UnderLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	maxCA := 3
	maxDB := 5

	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("acct-base").
		WillReturnRows(sqlmock.NewRows([]string{"name", "max_cloud_accounts", "max_database_connections"}).
			AddRow("base", maxCA, maxDB))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("acct-base").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("acct-base").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	resp, err := buildPlanLimitsResponse(db, "acct-base")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.CloudAccountsRemaining == nil || *resp.CloudAccountsRemaining != 2 {
		t.Errorf("expected cloud_accounts_remaining=2, got %v", resp.CloudAccountsRemaining)
	}
	if resp.DatabaseConnectionsRemaining == nil || *resp.DatabaseConnectionsRemaining != 2 {
		t.Errorf("expected database_connections_remaining=2, got %v", resp.DatabaseConnectionsRemaining)
	}
}

// TestBuildPlanLimitsResponse_RemainingCapacity_AtLimit verifies that remaining
// capacity is 0 (not negative) when usage equals the limit.
// Validates: Requirements 12.8
func TestBuildPlanLimitsResponse_RemainingCapacity_AtLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	maxCA := 1
	maxDB := 2

	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("acct-free").
		WillReturnRows(sqlmock.NewRows([]string{"name", "max_cloud_accounts", "max_database_connections"}).
			AddRow("free", maxCA, maxDB))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("acct-free").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1)) // at limit

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("acct-free").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2)) // at limit

	resp, err := buildPlanLimitsResponse(db, "acct-free")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.CloudAccountsRemaining == nil || *resp.CloudAccountsRemaining != 0 {
		t.Errorf("expected cloud_accounts_remaining=0 at limit, got %v", resp.CloudAccountsRemaining)
	}
	if resp.DatabaseConnectionsRemaining == nil || *resp.DatabaseConnectionsRemaining != 0 {
		t.Errorf("expected database_connections_remaining=0 at limit, got %v", resp.DatabaseConnectionsRemaining)
	}
}

// ─── HTTP handler tests ──────────────────────────────────────────────────────

// TestGetPlanLimits_Success verifies the HTTP handler returns 200 with plan data.
// Validates: Requirements 12.1, 12.5
func TestGetPlanLimits_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	maxCA := 10
	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("acct-pro").
		WillReturnRows(sqlmock.NewRows([]string{"name", "max_cloud_accounts", "max_database_connections"}).
			AddRow("pro", maxCA, nil))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("acct-pro").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("acct-pro").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	h := &PlanLimitsHandler{DB: db}
	r := newPlanLimitsRouter(h)

	w := getPlanLimits(r, "acct-pro")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp planLimitsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.PlanName != "pro" {
		t.Errorf("expected plan_name=pro, got %s", resp.PlanName)
	}
	if resp.CurrentCloudAccounts != 2 {
		t.Errorf("expected current_cloud_accounts=2, got %d", resp.CurrentCloudAccounts)
	}
	if resp.CurrentDatabaseConnections != 5 {
		t.Errorf("expected current_database_connections=5, got %d", resp.CurrentDatabaseConnections)
	}
}

// TestGetPlanLimits_MissingAccountID returns 401 when account_id is absent.
// Validates: Requirements 12.8
func TestGetPlanLimits_MissingAccountID(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	h := &PlanLimitsHandler{DB: db}
	// Router without account_id injection
	r := gin.New()
	r.GET("/billing/plan-limits", h.GetPlanLimits)

	req := httptest.NewRequest(http.MethodGet, "/billing/plan-limits", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── Error message content tests (Req 12.8) ─────────────────────────────────

// TestBuildPlanLimitsResponse_CloudAccountLimit_ErrorMessageContent verifies
// that the remaining capacity is 0 when at the cloud account limit, which
// signals the caller to return an upgrade-prompting error.
// Validates: Requirements 12.8
func TestBuildPlanLimitsResponse_CloudAccountLimit_Free_AtLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	maxCA := 1
	maxDB := 2

	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("acct-free").
		WillReturnRows(sqlmock.NewRows([]string{"name", "max_cloud_accounts", "max_database_connections"}).
			AddRow("free", maxCA, maxDB))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("acct-free").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("acct-free").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	resp, err := buildPlanLimitsResponse(db, "acct-free")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// At limit: remaining should be 0, signalling upgrade is needed
	if resp.CloudAccountsRemaining == nil || *resp.CloudAccountsRemaining != 0 {
		t.Errorf("expected cloud_accounts_remaining=0 when at Free limit, got %v", resp.CloudAccountsRemaining)
	}
	if resp.CurrentCloudAccounts != *resp.MaxCloudAccounts {
		t.Errorf("expected current (%d) == max (%d) when at limit", resp.CurrentCloudAccounts, *resp.MaxCloudAccounts)
	}
}

// TestBuildPlanLimitsResponse_DatabaseLimit_Base_AtLimit verifies that a Base
// plan account at its 5-database limit has 0 remaining connections.
// Validates: Requirements 12.6, 12.8
func TestBuildPlanLimitsResponse_DatabaseLimit_Base_AtLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	maxCA := 3
	maxDB := 5

	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("acct-base").
		WillReturnRows(sqlmock.NewRows([]string{"name", "max_cloud_accounts", "max_database_connections"}).
			AddRow("base", maxCA, maxDB))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("acct-base").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("acct-base").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	resp, err := buildPlanLimitsResponse(db, "acct-base")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.DatabaseConnectionsRemaining == nil || *resp.DatabaseConnectionsRemaining != 0 {
		t.Errorf("expected database_connections_remaining=0 when at Base limit, got %v", resp.DatabaseConnectionsRemaining)
	}
	if resp.CurrentDatabaseConnections != *resp.MaxDatabaseConnections {
		t.Errorf("expected current (%d) == max (%d) when at limit", resp.CurrentDatabaseConnections, *resp.MaxDatabaseConnections)
	}
}

// TestBuildPlanLimitsResponse_Pro_UnlimitedDatabases_HighUsage verifies that
// Pro plan accounts are never blocked on database connections regardless of count.
// Validates: Requirements 12.7
func TestBuildPlanLimitsResponse_Pro_UnlimitedDatabases_HighUsage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	maxCA := 10

	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("acct-pro").
		WillReturnRows(sqlmock.NewRows([]string{"name", "max_cloud_accounts", "max_database_connections"}).
			AddRow("pro", maxCA, nil))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("acct-pro").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("acct-pro").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(999))

	resp, err := buildPlanLimitsResponse(db, "acct-pro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Unlimited: nil means no cap, so no upgrade needed for databases
	if resp.MaxDatabaseConnections != nil {
		t.Errorf("expected max_database_connections=nil for Pro, got %d", *resp.MaxDatabaseConnections)
	}
	if resp.DatabaseConnectionsRemaining != nil {
		t.Errorf("expected database_connections_remaining=nil for unlimited Pro, got %d", *resp.DatabaseConnectionsRemaining)
	}
}

// TestBuildPlanLimitsResponse_Enterprise_UnlimitedAll_HighUsage verifies that
// Enterprise plan accounts are never blocked on any resource.
// Validates: Requirements 12.4, 12.7
func TestBuildPlanLimitsResponse_Enterprise_UnlimitedAll_HighUsage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("acct-ent").
		WillReturnRows(sqlmock.NewRows([]string{"name", "max_cloud_accounts", "max_database_connections"}).
			AddRow("enterprise", nil, nil))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("acct-ent").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(500))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("acct-ent").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1000))

	resp, err := buildPlanLimitsResponse(db, "acct-ent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.MaxCloudAccounts != nil {
		t.Errorf("expected max_cloud_accounts=nil for Enterprise, got %d", *resp.MaxCloudAccounts)
	}
	if resp.MaxDatabaseConnections != nil {
		t.Errorf("expected max_database_connections=nil for Enterprise, got %d", *resp.MaxDatabaseConnections)
	}
	if resp.CloudAccountsRemaining != nil {
		t.Errorf("expected cloud_accounts_remaining=nil for Enterprise, got %d", *resp.CloudAccountsRemaining)
	}
	if resp.DatabaseConnectionsRemaining != nil {
		t.Errorf("expected database_connections_remaining=nil for Enterprise, got %d", *resp.DatabaseConnectionsRemaining)
	}
}
