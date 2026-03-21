package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

func setupCostRouter(h *CostHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/finops/costs/summary", func(c *gin.Context) {
		c.Set("account_id", "account-1")
		h.GetCostSummary(c)
	})
	return r
}

// TestGetCostSummary_ByProvider verifies that costs are correctly summed per cloud provider.
// Validates: Requirements 4.6
func TestGetCostSummary_ByProvider(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// Total cost query
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1500.00))

	// Provider breakdown query
	mock.ExpectQuery(`SELECT ca\.provider, COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"provider", "total"}).
			AddRow("aws", 1000.00).
			AddRow("azure", 500.00))

	// Service breakdown query
	mock.ExpectQuery(`SELECT cc\.service_name, COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"service_name", "total"}).
			AddRow("EC2", 800.00).
			AddRow("Blob Storage", 500.00))

	// Daily breakdown query
	mock.ExpectQuery(`SELECT cc\.date, COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"date", "total"}).
			AddRow("2024-01-01", 1500.00))

	h := &CostHandler{DB: db}
	router := setupCostRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/finops/costs/summary", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	byProvider, ok := resp["breakdown_by_provider"].(map[string]any)
	if !ok {
		t.Fatal("breakdown_by_provider missing or wrong type")
	}
	if byProvider["aws"] != 1000.00 {
		t.Errorf("expected aws=1000.00, got %v", byProvider["aws"])
	}
	if byProvider["azure"] != 500.00 {
		t.Errorf("expected azure=500.00, got %v", byProvider["azure"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

// TestGetCostSummary_ByService verifies that costs are correctly summed per service.
// Validates: Requirements 4.6
func TestGetCostSummary_ByService(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(3000.00))

	mock.ExpectQuery(`SELECT ca\.provider, COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"provider", "total"}).
			AddRow("aws", 3000.00))

	mock.ExpectQuery(`SELECT cc\.service_name, COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"service_name", "total"}).
			AddRow("EC2", 2000.00).
			AddRow("S3", 700.00).
			AddRow("Lambda", 300.00))

	mock.ExpectQuery(`SELECT cc\.date, COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"date", "total"}))

	h := &CostHandler{DB: db}
	router := setupCostRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/finops/costs/summary", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	services, ok := resp["breakdown_by_service"].([]any)
	if !ok {
		t.Fatal("breakdown_by_service missing or wrong type")
	}
	if len(services) != 3 {
		t.Fatalf("expected 3 services, got %d", len(services))
	}

	// First entry should be EC2 (highest cost, ordered DESC)
	first := services[0].(map[string]any)
	if first["service"] != "EC2" {
		t.Errorf("expected first service=EC2, got %v", first["service"])
	}
	if first["cost"] != 2000.00 {
		t.Errorf("expected EC2 cost=2000.00, got %v", first["cost"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

// TestGetCostSummary_DateRangeFilter verifies that date range params are passed to queries.
// Validates: Requirements 4.6
func TestGetCostSummary_DateRangeFilter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// All queries should receive account-1, start_date, end_date
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1", "2024-01-01", "2024-01-31").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(500.00))

	mock.ExpectQuery(`SELECT ca\.provider, COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1", "2024-01-01", "2024-01-31").
		WillReturnRows(sqlmock.NewRows([]string{"provider", "total"}).
			AddRow("gcp", 500.00))

	mock.ExpectQuery(`SELECT cc\.service_name, COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1", "2024-01-01", "2024-01-31").
		WillReturnRows(sqlmock.NewRows([]string{"service_name", "total"}).
			AddRow("BigQuery", 500.00))

	mock.ExpectQuery(`SELECT cc\.date, COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1", "2024-01-01", "2024-01-31").
		WillReturnRows(sqlmock.NewRows([]string{"date", "total"}).
			AddRow("2024-01-15", 500.00))

	h := &CostHandler{DB: db}
	router := setupCostRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/finops/costs/summary?start_date=2024-01-01&end_date=2024-01-31", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["total_cost"] != 500.00 {
		t.Errorf("expected total_cost=500.00, got %v", resp["total_cost"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

// TestGetCostSummary_EmptyResult verifies that zero costs are returned when no data exists.
// Validates: Requirements 4.6
func TestGetCostSummary_EmptyResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0.00))

	mock.ExpectQuery(`SELECT ca\.provider, COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"provider", "total"}))

	mock.ExpectQuery(`SELECT cc\.service_name, COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"service_name", "total"}))

	mock.ExpectQuery(`SELECT cc\.date, COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"date", "total"}))

	h := &CostHandler{DB: db}
	router := setupCostRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/finops/costs/summary", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["total_cost"] != 0.00 {
		t.Errorf("expected total_cost=0, got %v", resp["total_cost"])
	}

	byProvider, ok := resp["breakdown_by_provider"].(map[string]any)
	if !ok {
		t.Fatal("breakdown_by_provider missing or wrong type")
	}
	if len(byProvider) != 0 {
		t.Errorf("expected empty provider breakdown, got %v", byProvider)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

// TestGetCostSummary_MultipleProviders verifies that multiple providers are all aggregated correctly.
// Validates: Requirements 4.6
func TestGetCostSummary_MultipleProviders(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(15420.50))

	mock.ExpectQuery(`SELECT ca\.provider, COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"provider", "total"}).
			AddRow("aws", 10200.00).
			AddRow("azure", 3120.50).
			AddRow("gcp", 2100.00))

	mock.ExpectQuery(`SELECT cc\.service_name, COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"service_name", "total"}).
			AddRow("EC2", 5000.00).
			AddRow("S3", 1200.00))

	mock.ExpectQuery(`SELECT cc\.date, COALESCE\(SUM\(cc\.cost_amount\), 0\)`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{"date", "total"}))

	h := &CostHandler{DB: db}
	router := setupCostRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/finops/costs/summary", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["total_cost"] != 15420.50 {
		t.Errorf("expected total_cost=15420.50, got %v", resp["total_cost"])
	}

	byProvider := resp["breakdown_by_provider"].(map[string]any)
	if len(byProvider) != 3 {
		t.Errorf("expected 3 providers, got %d", len(byProvider))
	}
	if byProvider["aws"] != 10200.00 {
		t.Errorf("expected aws=10200.00, got %v", byProvider["aws"])
	}
	if byProvider["azure"] != 3120.50 {
		t.Errorf("expected azure=3120.50, got %v", byProvider["azure"])
	}
	if byProvider["gcp"] != 2100.00 {
		t.Errorf("expected gcp=2100.00, got %v", byProvider["gcp"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

// TestGetCostSummary_Unauthorized verifies that missing account_id returns 401.
// Validates: Requirements 4.6
func TestGetCostSummary_Unauthorized(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &CostHandler{DB: db}
	// No account_id set in context
	r.GET("/finops/costs/summary", h.GetCostSummary)

	req := httptest.NewRequest(http.MethodGet, "/finops/costs/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
