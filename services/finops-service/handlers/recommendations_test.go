package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

func setupRecommendationRouter(h *RecommendationHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/finops/recommendations", func(c *gin.Context) {
		c.Set("account_id", "account-1")
		h.GetRecommendations(c)
	})
	return r
}

// TestDetectIdleResources_FlagsZeroUsageFor7Days verifies that a resource with 0% usage
// for 7 consecutive days is detected as idle and inserted as a recommendation.
// Validates: Requirements 22.1
func TestDetectIdleResources_FlagsZeroUsageFor7Days(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// Expect the idle resource query to return one resource with 0 cost for 7 days
	mock.ExpectQuery(`SELECT resource_id, service_name, AVG\(cost_amount\)`).
		WithArgs("cloud-acct-1", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"resource_id", "service_name", "avg_cost"}).
			AddRow("i-1234567890", "EC2", 0.0))

	// Expect the INSERT for the idle recommendation
	mock.ExpectExec(`INSERT INTO cost_recommendations`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	scheduler := &RecommendationScheduler{DB: db}
	scheduler.detectIdleResources("cloud-acct-1")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

// TestDetectIdleResources_DoesNotFlagActiveResource verifies that a resource with recent
// non-zero usage is NOT flagged as idle.
// Validates: Requirements 22.1
func TestDetectIdleResources_DoesNotFlagActiveResource(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// Query returns no rows (active resource doesn't match the zero-cost filter)
	mock.ExpectQuery(`SELECT resource_id, service_name, AVG\(cost_amount\)`).
		WithArgs("cloud-acct-1", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"resource_id", "service_name", "avg_cost"}))

	// No INSERT should happen
	scheduler := &RecommendationScheduler{DB: db}
	scheduler.detectIdleResources("cloud-acct-1")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

// TestDetectOversizedResources_FlagsLowUtilizationResource verifies that a resource with
// average utilization below 20% over 30 days is flagged as oversized.
// Validates: Requirements 22.2
func TestDetectOversizedResources_FlagsLowUtilizationResource(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// avg/max = 10/100 = 0.10 < 0.20 → oversized
	mock.ExpectQuery(`SELECT resource_id, service_name`).
		WithArgs("cloud-acct-1", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"resource_id", "service_name", "avg_daily_cost", "max_daily_cost"}).
			AddRow("i-oversized-01", "EC2", 10.0, 100.0))

	// Expect INSERT for the oversized recommendation
	mock.ExpectExec(`INSERT INTO cost_recommendations`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	scheduler := &RecommendationScheduler{DB: db}
	scheduler.detectOversizedResources("cloud-acct-1")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

// TestDetectOversizedResources_DoesNotFlagHighUtilizationResource verifies that a resource
// with utilization above 20% is NOT flagged as oversized.
// Validates: Requirements 22.2
func TestDetectOversizedResources_DoesNotFlagHighUtilizationResource(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// Query returns no rows (high utilization resource filtered out by HAVING clause)
	mock.ExpectQuery(`SELECT resource_id, service_name`).
		WithArgs("cloud-acct-1", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"resource_id", "service_name", "avg_daily_cost", "max_daily_cost"}))

	// No INSERT should happen
	scheduler := &RecommendationScheduler{DB: db}
	scheduler.detectOversizedResources("cloud-acct-1")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

// TestDetectOversizedResources_SavingsCalculation verifies that potential monthly savings
// are calculated as (maxCost - avgCost) * 30.
// Validates: Requirements 22.4
func TestDetectOversizedResources_SavingsCalculation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	avgCost := 5.0
	maxCost := 50.0
	expectedSavings := (maxCost - avgCost) * 30 // = 1350.0

	mock.ExpectQuery(`SELECT resource_id, service_name`).
		WithArgs("cloud-acct-1", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"resource_id", "service_name", "avg_daily_cost", "max_daily_cost"}).
			AddRow("i-savings-test", "EC2", avgCost, maxCost))

	// Capture the INSERT to verify the savings value.
	// The INSERT uses NOW() inline so only 7 bound args: id, cloud_account_id,
	// recommendation_type, resource_id, service_name, description, potential_monthly_savings.
	mock.ExpectExec(`INSERT INTO cost_recommendations`).
		WithArgs(
			sqlmock.AnyArg(), // id
			"cloud-acct-1",
			"oversized_resource",
			"i-savings-test",
			"EC2",
			sqlmock.AnyArg(), // description
			expectedSavings,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	scheduler := &RecommendationScheduler{DB: db}
	scheduler.detectOversizedResources("cloud-acct-1")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations (savings=%.2f expected): %v", expectedSavings, err)
	}
}

// TestGetRecommendations_SortedBySavingsDescending verifies that recommendations are
// returned sorted by potential_monthly_savings in descending order (highest first).
// Validates: Requirements 22.4, 22.5
func TestGetRecommendations_SortedBySavingsDescending(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// The query uses ORDER BY cr.potential_monthly_savings DESC
	mock.ExpectQuery(`SELECT cr\.id, cr\.cloud_account_id`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "cloud_account_id", "recommendation_type", "resource_id",
			"service_name", "description", "potential_monthly_savings", "created_at",
		}).
			AddRow("rec-1", "ca-1", "oversized_resource", "i-001", "EC2", "Low utilization", 1350.00, "2024-01-01 02:00:00").
			AddRow("rec-2", "ca-1", "idle_resource", "i-002", "EC2", "Zero usage 7 days", 150.00, "2024-01-01 02:00:00").
			AddRow("rec-3", "ca-1", "unattached_storage", "vol-003", "EBS", "Unattached volume", 30.00, "2024-01-01 02:00:00"))

	h := &RecommendationHandler{DB: db}
	router := setupRecommendationRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/finops/recommendations", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	recs, ok := resp["recommendations"].([]any)
	if !ok {
		t.Fatal("recommendations missing or wrong type")
	}
	if len(recs) != 3 {
		t.Fatalf("expected 3 recommendations, got %d", len(recs))
	}

	// Verify descending order by savings
	first := recs[0].(map[string]any)
	second := recs[1].(map[string]any)
	third := recs[2].(map[string]any)

	if first["potential_monthly_savings"].(float64) != 1350.00 {
		t.Errorf("expected first savings=1350.00, got %v", first["potential_monthly_savings"])
	}
	if second["potential_monthly_savings"].(float64) != 150.00 {
		t.Errorf("expected second savings=150.00, got %v", second["potential_monthly_savings"])
	}
	if third["potential_monthly_savings"].(float64) != 30.00 {
		t.Errorf("expected third savings=30.00, got %v", third["potential_monthly_savings"])
	}

	// Verify total savings is the sum
	totalSavings, ok := resp["total_potential_savings"].(float64)
	if !ok {
		t.Fatal("total_potential_savings missing or wrong type")
	}
	expectedTotal := 1350.00 + 150.00 + 30.00
	if totalSavings != expectedTotal {
		t.Errorf("expected total_potential_savings=%.2f, got %.2f", expectedTotal, totalSavings)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

// TestGetRecommendations_Unauthorized verifies that missing account_id returns 401.
// Validates: Requirements 22.1, 22.2
func TestGetRecommendations_Unauthorized(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &RecommendationHandler{DB: db}
	// No account_id set in context
	r.GET("/finops/recommendations", h.GetRecommendations)

	req := httptest.NewRequest(http.MethodGet, "/finops/recommendations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestGetRecommendations_EmptyResult verifies that an empty list and zero total savings
// are returned when no recommendations exist.
// Validates: Requirements 22.4
func TestGetRecommendations_EmptyResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT cr\.id, cr\.cloud_account_id`).
		WithArgs("account-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "cloud_account_id", "recommendation_type", "resource_id",
			"service_name", "description", "potential_monthly_savings", "created_at",
		}))

	h := &RecommendationHandler{DB: db}
	router := setupRecommendationRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/finops/recommendations", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["total_potential_savings"].(float64) != 0.0 {
		t.Errorf("expected total_potential_savings=0, got %v", resp["total_potential_savings"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}
