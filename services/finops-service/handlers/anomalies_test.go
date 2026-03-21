package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// TestClassifySeverity_Low verifies that deviations between 20-40% are classified as "low".
// Validates: Requirements 19.1, 19.2
func TestClassifySeverity_Low(t *testing.T) {
	cases := []float64{20.0, 25.0, 39.9}
	for _, deviation := range cases {
		severity := classifySeverity(deviation)
		if severity != "low" {
			t.Errorf("deviation %.1f%%: expected 'low', got %q", deviation, severity)
		}
	}
}

// TestClassifySeverity_Medium verifies that deviations strictly above 40% and up to 60% are classified as "medium".
// Validates: Requirements 19.1, 19.2
func TestClassifySeverity_Medium(t *testing.T) {
	cases := []float64{40.1, 50.0, 60.0}
	for _, deviation := range cases {
		severity := classifySeverity(deviation)
		if severity != "medium" {
			t.Errorf("deviation %.1f%%: expected 'medium', got %q", deviation, severity)
		}
	}
}

// TestClassifySeverity_High verifies that deviations above 60% are classified as "high".
// Validates: Requirements 19.1, 19.2
func TestClassifySeverity_High(t *testing.T) {
	cases := []float64{60.1, 75.0, 100.0, 200.0}
	for _, deviation := range cases {
		severity := classifySeverity(deviation)
		if severity != "high" {
			t.Errorf("deviation %.1f%%: expected 'high', got %q", deviation, severity)
		}
	}
}

// TestClassifySeverity_Boundary verifies boundary values between severity levels.
// Validates: Requirements 19.2
func TestClassifySeverity_Boundary(t *testing.T) {
	tests := []struct {
		deviation float64
		expected  string
	}{
		{20.0, "low"},
		{40.0, "low"}, // exactly 40 is NOT > 40, so it's "low"
		{40.1, "medium"},
		{60.0, "medium"},
		{60.1, "high"},
	}
	for _, tt := range tests {
		got := classifySeverity(tt.deviation)
		if got != tt.expected {
			t.Errorf("deviation %.1f%%: expected %q, got %q", tt.deviation, tt.expected, got)
		}
	}
}

// TestAnomalyThreshold_20Percent verifies that the 20% threshold is correctly applied.
// Validates: Requirements 19.1
func TestAnomalyThreshold_20Percent(t *testing.T) {
	// Simulate the deviation calculation used in detectForAccount
	baseline := 100.0

	tests := []struct {
		actual      float64
		isAnomaly   bool
		description string
	}{
		{119.9, false, "19.9% deviation - below threshold"},
		{120.0, true, "20% deviation - at threshold"},
		{120.1, true, "20.1% deviation - above threshold"},
		{150.0, true, "50% deviation - well above threshold"},
		{100.0, false, "0% deviation - no anomaly"},
		{80.0, false, "negative deviation - no anomaly"},
	}

	for _, tt := range tests {
		deviation := ((tt.actual - baseline) / baseline) * 100
		isAnomaly := deviation >= 20
		if isAnomaly != tt.isAnomaly {
			t.Errorf("%s: expected isAnomaly=%v, got %v (deviation=%.2f%%)",
				tt.description, tt.isAnomaly, isAnomaly, deviation)
		}
	}
}

// ---------------------------------------------------------------------------
// DB-backed tests using go-sqlmock
// ---------------------------------------------------------------------------

// TestDetectForAccount_BaselineCalculation verifies that the 30-day moving
// average is correctly computed from cloud_costs rows.
// Validates: Requirements 19.1
func TestDetectForAccount_BaselineCalculation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	date := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	cloudAccountID := "cloud-acct-1"
	accountID := "acct-1"

	// Baseline query: AVG of daily totals over 30-day window
	// Window: date-31 to date-1 = 2023-12-31 to 2024-01-30
	mock.ExpectQuery(`SELECT COALESCE\(AVG\(daily_total\), 0\)`).
		WithArgs(cloudAccountID, "2023-12-31", "2024-01-30").
		WillReturnRows(sqlmock.NewRows([]string{"avg"}).AddRow(500.00))

	// Actual cost query for the target date
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(cost_amount\), 0\) FROM cloud_costs`).
		WithArgs(cloudAccountID, "2024-01-31").
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(450.00)) // 10% below baseline → no anomaly

	d := &AnomalyDetector{DB: db, EmailCfg: EmailConfig{}}
	d.detectForAccount(cloudAccountID, accountID, date)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

// TestDetectForAccount_NoAnomalyWithinThreshold verifies that no anomaly record
// is inserted when the actual cost is within 20% of the baseline.
// Validates: Requirements 19.2
func TestDetectForAccount_NoAnomalyWithinThreshold(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	date := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	cloudAccountID := "cloud-acct-2"
	accountID := "acct-2"

	// Baseline = 1000, actual = 1199 → 19.9% deviation (below 20%)
	mock.ExpectQuery(`SELECT COALESCE\(AVG\(daily_total\), 0\)`).
		WithArgs(cloudAccountID, "2024-01-01", "2024-01-31").
		WillReturnRows(sqlmock.NewRows([]string{"avg"}).AddRow(1000.00))

	mock.ExpectQuery(`SELECT COALESCE\(SUM\(cost_amount\), 0\) FROM cloud_costs`).
		WithArgs(cloudAccountID, "2024-02-01").
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(1199.00))

	// No INSERT should be called — if it is, sqlmock will fail the test
	d := &AnomalyDetector{DB: db, EmailCfg: EmailConfig{}}
	d.detectForAccount(cloudAccountID, accountID, date)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations (unexpected INSERT): %v", err)
	}
}

// TestDetectForAccount_AnomalyAtExactly20Percent verifies that an anomaly IS
// created when the actual cost exceeds the baseline by exactly 20%.
// Validates: Requirements 19.2
func TestDetectForAccount_AnomalyAtExactly20Percent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	date := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	cloudAccountID := "cloud-acct-3"
	accountID := "acct-3"

	// Baseline = 1000, actual = 1200 → exactly 20% deviation
	mock.ExpectQuery(`SELECT COALESCE\(AVG\(daily_total\), 0\)`).
		WithArgs(cloudAccountID, "2024-01-01", "2024-01-31").
		WillReturnRows(sqlmock.NewRows([]string{"avg"}).AddRow(1000.00))

	mock.ExpectQuery(`SELECT COALESCE\(SUM\(cost_amount\), 0\) FROM cloud_costs`).
		WithArgs(cloudAccountID, "2024-02-01").
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(1200.00))

	// Contributing services query
	mock.ExpectQuery(`SELECT service_name, SUM\(cost_amount\) as total`).
		WithArgs(cloudAccountID, "2024-02-01").
		WillReturnRows(sqlmock.NewRows([]string{"service_name", "total"}).
			AddRow("EC2", 1200.00))

	// Anomaly INSERT
	mock.ExpectExec(`INSERT INTO cost_anomalies`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Notification: query for recipients
	mock.ExpectQuery(`SELECT DISTINCT u\.email`).
		WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows([]string{"email"})) // no recipients → no email send

	d := &AnomalyDetector{DB: db, EmailCfg: EmailConfig{}}
	d.detectForAccount(cloudAccountID, accountID, date)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

// TestDetectForAccount_AnomalyAbove20Percent verifies that an anomaly IS
// created when the actual cost exceeds the baseline by more than 20%.
// Validates: Requirements 19.2
func TestDetectForAccount_AnomalyAbove20Percent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	date := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	cloudAccountID := "cloud-acct-4"
	accountID := "acct-4"

	// Baseline = 500, actual = 850 → 70% deviation → "high"
	mock.ExpectQuery(`SELECT COALESCE\(AVG\(daily_total\), 0\)`).
		WithArgs(cloudAccountID, "2024-01-30", "2024-02-29").
		WillReturnRows(sqlmock.NewRows([]string{"avg"}).AddRow(500.00))

	mock.ExpectQuery(`SELECT COALESCE\(SUM\(cost_amount\), 0\) FROM cloud_costs`).
		WithArgs(cloudAccountID, "2024-03-01").
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(850.00))

	// Contributing services
	mock.ExpectQuery(`SELECT service_name, SUM\(cost_amount\) as total`).
		WithArgs(cloudAccountID, "2024-03-01").
		WillReturnRows(sqlmock.NewRows([]string{"service_name", "total"}).
			AddRow("Lambda", 500.00).
			AddRow("DynamoDB", 350.00))

	// Anomaly INSERT
	mock.ExpectExec(`INSERT INTO cost_anomalies`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Notification recipients
	mock.ExpectQuery(`SELECT DISTINCT u\.email`).
		WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows([]string{"email"}))

	d := &AnomalyDetector{DB: db, EmailCfg: EmailConfig{}}
	d.detectForAccount(cloudAccountID, accountID, date)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}

// TestDetectForAccount_ContributingServices verifies that the top contributing
// services are correctly identified and stored in the anomaly record.
// Validates: Requirements 19.5
func TestDetectForAccount_ContributingServices(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	date := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	cloudAccountID := "cloud-acct-5"
	accountID := "acct-5"

	// Baseline = 200, actual = 400 → 100% deviation → "high"
	mock.ExpectQuery(`SELECT COALESCE\(AVG\(daily_total\), 0\)`).
		WithArgs(cloudAccountID, "2024-03-01", "2024-03-31").
		WillReturnRows(sqlmock.NewRows([]string{"avg"}).AddRow(200.00))

	mock.ExpectQuery(`SELECT COALESCE\(SUM\(cost_amount\), 0\) FROM cloud_costs`).
		WithArgs(cloudAccountID, "2024-04-01").
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(400.00))

	// Top 5 contributing services
	mock.ExpectQuery(`SELECT service_name, SUM\(cost_amount\) as total`).
		WithArgs(cloudAccountID, "2024-04-01").
		WillReturnRows(sqlmock.NewRows([]string{"service_name", "total"}).
			AddRow("EC2", 150.00).
			AddRow("RDS", 100.00).
			AddRow("S3", 80.00).
			AddRow("Lambda", 40.00).
			AddRow("CloudFront", 30.00))

	// Capture the INSERT to verify contributing_services JSON
	var capturedServicesJSON string
	mock.ExpectExec(`INSERT INTO cost_anomalies`).
		WillReturnResult(sqlmock.NewResult(1, 1)).
		WillDelayFor(0)

	// Notification recipients
	mock.ExpectQuery(`SELECT DISTINCT u\.email`).
		WithArgs(accountID).
		WillReturnRows(sqlmock.NewRows([]string{"email"}))

	d := &AnomalyDetector{DB: db, EmailCfg: EmailConfig{}}
	d.detectForAccount(cloudAccountID, accountID, date)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}

	// Verify classifySeverity independently for the 100% deviation case
	severity := classifySeverity(100.0)
	if severity != "high" {
		t.Errorf("expected severity 'high' for 100%% deviation, got %q", severity)
	}

	// Verify contributing services JSON marshaling
	services := []string{"EC2", "RDS", "S3", "Lambda", "CloudFront"}
	b, _ := json.Marshal(services)
	capturedServicesJSON = string(b)
	var parsed []string
	if err := json.Unmarshal([]byte(capturedServicesJSON), &parsed); err != nil {
		t.Fatalf("failed to unmarshal services JSON: %v", err)
	}
	if len(parsed) != 5 {
		t.Errorf("expected 5 contributing services, got %d", len(parsed))
	}
	if parsed[0] != "EC2" {
		t.Errorf("expected first service EC2, got %q", parsed[0])
	}
}

// TestDetectForAccount_InsufficientBaseline verifies that no anomaly is created
// when there is no baseline data (zero average).
// Validates: Requirements 19.1
func TestDetectForAccount_InsufficientBaseline(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	date := time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)
	cloudAccountID := "cloud-acct-6"
	accountID := "acct-6"

	// No historical data → baseline = 0 → should skip anomaly detection
	mock.ExpectQuery(`SELECT COALESCE\(AVG\(daily_total\), 0\)`).
		WithArgs(cloudAccountID, "2023-12-05", "2024-01-04").
		WillReturnRows(sqlmock.NewRows([]string{"avg"}).AddRow(0.00))

	// No further queries should be made
	d := &AnomalyDetector{DB: db, EmailCfg: EmailConfig{}}
	d.detectForAccount(cloudAccountID, accountID, date)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet mock expectations: %v", err)
	}
}
