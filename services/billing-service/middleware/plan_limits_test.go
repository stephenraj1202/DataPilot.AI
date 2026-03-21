package middleware

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

func setupRouter(db *sql.DB, resourceType ResourceType) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/test", func(c *gin.Context) {
		// Simulate auth middleware setting account_id
		c.Set("account_id", "test-account-id")
		c.Next()
	}, CheckPlanLimit(db, resourceType), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})
	return r
}

func setupRouterWithAccount(db *sql.DB, resourceType ResourceType, accountID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/test", func(c *gin.Context) {
		if accountID != "" {
			c.Set("account_id", accountID)
		}
		c.Next()
	}, CheckPlanLimit(db, resourceType), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})
	return r
}

// TestCheckPlanLimit_CloudAccount_UnderLimit verifies that a request passes when under the cloud account limit.
func TestCheckPlanLimit_CloudAccount_UnderLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	maxAccounts := 3 // base plan
	// Subscription query
	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("test-account-id").
		WillReturnRows(sqlmock.NewRows([]string{"name", "max_cloud_accounts", "max_database_connections"}).
			AddRow("base", &maxAccounts, nil))
	// Current cloud accounts count
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("test-account-id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	// Current database connections count
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("test-account-id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	r := setupRouter(db, ResourceCloudAccount)
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCheckPlanLimit_CloudAccount_AtLimit verifies HTTP 403 is returned when the cloud account limit is reached.
func TestCheckPlanLimit_CloudAccount_AtLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	maxAccounts := 1 // free plan
	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("test-account-id").
		WillReturnRows(sqlmock.NewRows([]string{"name", "max_cloud_accounts", "max_database_connections"}).
			AddRow("free", &maxAccounts, nil))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("test-account-id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1)) // already at limit
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("test-account-id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	r := setupRouter(db, ResourceCloudAccount)
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if body["upgrade_required"] != true {
		t.Errorf("expected upgrade_required=true in response")
	}
	if body["resource_type"] != string(ResourceCloudAccount) {
		t.Errorf("expected resource_type=%s, got %v", ResourceCloudAccount, body["resource_type"])
	}
}

// TestCheckPlanLimit_DatabaseConnection_AtLimit verifies HTTP 403 when database connection limit is reached.
func TestCheckPlanLimit_DatabaseConnection_AtLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	maxDB := 2 // free plan
	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("test-account-id").
		WillReturnRows(sqlmock.NewRows([]string{"name", "max_cloud_accounts", "max_database_connections"}).
			AddRow("free", nil, &maxDB))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("test-account-id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("test-account-id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2)) // at limit

	r := setupRouter(db, ResourceDatabaseConnection)
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if body["resource_type"] != string(ResourceDatabaseConnection) {
		t.Errorf("expected resource_type=%s, got %v", ResourceDatabaseConnection, body["resource_type"])
	}
}

// TestCheckPlanLimit_Unlimited_Enterprise verifies that nil limits (Enterprise/Pro) always pass.
func TestCheckPlanLimit_Unlimited_Enterprise(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// Enterprise plan: both limits are NULL (unlimited)
	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("test-account-id").
		WillReturnRows(sqlmock.NewRows([]string{"name", "max_cloud_accounts", "max_database_connections"}).
			AddRow("enterprise", nil, nil))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("test-account-id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("test-account-id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(50))

	r := setupRouter(db, ResourceCloudAccount)
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for enterprise plan, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCheckPlanLimit_NoAccountID verifies HTTP 401 when account_id is missing from context.
func TestCheckPlanLimit_NoAccountID(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	r := setupRouterWithAccount(db, ResourceCloudAccount, "") // no account_id set
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCheckPlanLimit_ProPlan_CloudAccountLimit verifies Pro plan allows up to 10 cloud accounts.
func TestCheckPlanLimit_ProPlan_CloudAccountLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	maxAccounts := 10 // pro plan
	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("test-account-id").
		WillReturnRows(sqlmock.NewRows([]string{"name", "max_cloud_accounts", "max_database_connections"}).
			AddRow("pro", &maxAccounts, nil))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("test-account-id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10)) // at limit
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("test-account-id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	r := setupRouter(db, ResourceCloudAccount)
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for pro plan at limit, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCheckPlanLimit_NoSubscription_DefaultsToFree verifies that accounts with no subscription default to free plan limits.
func TestCheckPlanLimit_NoSubscription_DefaultsToFree(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// No subscription found
	mock.ExpectQuery(`SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections`).
		WithArgs("test-account-id").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM cloud_accounts`).
		WithArgs("test-account-id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1)) // at free limit
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM database_connections`).
		WithArgs("test-account-id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	r := setupRouter(db, ResourceCloudAccount)
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should be blocked at free plan limit of 1
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 (defaulted to free plan), got %d: %s", w.Code, w.Body.String())
	}
}
