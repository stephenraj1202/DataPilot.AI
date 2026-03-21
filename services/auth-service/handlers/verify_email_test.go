package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestVerifyEmail_Success(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	handler := &VerifyEmailHandler{DB: db}
	router := gin.New()
	router.GET("/auth/verify-email", handler.VerifyEmail)

	// Test data
	token := "valid-token-123"
	userID := "user-uuid-123"
	futureExpiry := time.Now().Add(1 * time.Hour)

	// Mock expectations
	mock.ExpectQuery("SELECT id, email_verified, verification_token_expiry FROM users").
		WithArgs(token).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email_verified", "verification_token_expiry"}).
			AddRow(userID, false, futureExpiry))

	mock.ExpectExec("UPDATE users SET email_verified").
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Execute request
	req, _ := http.NewRequest("GET", "/auth/verify-email?token="+token, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response VerifyEmailResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "Email verified successfully", response.Message)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifyEmail_AlreadyVerified(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	handler := &VerifyEmailHandler{DB: db}
	router := gin.New()
	router.GET("/auth/verify-email", handler.VerifyEmail)

	// Test data
	token := "valid-token-123"
	userID := "user-uuid-123"
	futureExpiry := time.Now().Add(1 * time.Hour)

	// Mock expectations - user already verified
	mock.ExpectQuery("SELECT id, email_verified, verification_token_expiry FROM users").
		WithArgs(token).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email_verified", "verification_token_expiry"}).
			AddRow(userID, true, futureExpiry))

	// Execute request
	req, _ := http.NewRequest("GET", "/auth/verify-email?token="+token, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response VerifyEmailResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "Email already verified", response.Message)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifyEmail_InvalidToken(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	handler := &VerifyEmailHandler{DB: db}
	router := gin.New()
	router.GET("/auth/verify-email", handler.VerifyEmail)

	// Test data
	token := "invalid-token"

	// Mock expectations - no user found
	mock.ExpectQuery("SELECT id, email_verified, verification_token_expiry FROM users").
		WithArgs(token).
		WillReturnError(sql.ErrNoRows)

	// Execute request
	req, _ := http.NewRequest("GET", "/auth/verify-email?token="+token, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response["success"].(bool))
	assert.Equal(t, "Invalid verification token", response["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifyEmail_ExpiredToken(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	handler := &VerifyEmailHandler{DB: db}
	router := gin.New()
	router.GET("/auth/verify-email", handler.VerifyEmail)

	// Test data
	token := "expired-token-123"
	userID := "user-uuid-123"
	pastExpiry := time.Now().Add(-1 * time.Hour) // Expired 1 hour ago

	// Mock expectations
	mock.ExpectQuery("SELECT id, email_verified, verification_token_expiry FROM users").
		WithArgs(token).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email_verified", "verification_token_expiry"}).
			AddRow(userID, false, pastExpiry))

	// Execute request
	req, _ := http.NewRequest("GET", "/auth/verify-email?token="+token, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response["success"].(bool))
	assert.Equal(t, "Verification token has expired", response["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifyEmail_MissingToken(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	handler := &VerifyEmailHandler{DB: db}
	router := gin.New()
	router.GET("/auth/verify-email", handler.VerifyEmail)

	// Execute request without token
	req, _ := http.NewRequest("GET", "/auth/verify-email", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response["success"].(bool))
	assert.Equal(t, "Verification token is required", response["error"])
}
