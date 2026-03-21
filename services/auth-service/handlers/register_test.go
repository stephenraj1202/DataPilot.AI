package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
)

// MockEmailSender implements EmailSender interface for testing
type MockEmailSender struct {
	SendCalled bool
	SendError  error
}

func (m *MockEmailSender) SendVerificationEmail(toEmail, verificationToken string) error {
	m.SendCalled = true
	return m.SendError
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "Valid password",
			password: "SecurePass123!",
			wantErr:  false,
		},
		{
			name:     "Too short",
			password: "Short1!",
			wantErr:  true,
			errMsg:   "password must be at least 12 characters long",
		},
		{
			name:     "No uppercase",
			password: "securepass123!",
			wantErr:  true,
			errMsg:   "password must contain at least one uppercase letter",
		},
		{
			name:     "No lowercase",
			password: "SECUREPASS123!",
			wantErr:  true,
			errMsg:   "password must contain at least one lowercase letter",
		},
		{
			name:     "No number",
			password: "SecurePass!@#",
			wantErr:  true,
			errMsg:   "password must contain at least one number",
		},
		{
			name:     "No special character",
			password: "SecurePass123",
			wantErr:  true,
			errMsg:   "password must contain at least one special character",
		},
		{
			name:     "Common password",
			password: "Password123!",
			wantErr:  true,
			errMsg:   "password is too common, please choose a stronger password",
		},
		{
			name:     "Another common password",
			password: "Welcome1234!",
			wantErr:  true,
			errMsg:   "password is too common, please choose a stronger password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePassword(tt.password)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsCommonPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{"password", "password", true},
		{"Password", "Password", true},
		{"PASSWORD", "PASSWORD", true},
		{"123456", "123456", true},
		{"qwerty", "qwerty", true},
		{"admin", "admin", true},
		{"NotCommon123!", "NotCommon123!", false},
		{"MySecureP@ss123", "MySecureP@ss123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCommonPassword(tt.password)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGenerateSecureToken(t *testing.T) {
	token1, err1 := generateSecureToken(32)
	assert.NoError(t, err1)
	assert.NotEmpty(t, token1)
	assert.Equal(t, 64, len(token1)) // 32 bytes = 64 hex characters

	token2, err2 := generateSecureToken(32)
	assert.NoError(t, err2)
	assert.NotEmpty(t, token2)
	assert.NotEqual(t, token1, token2) // Tokens should be unique
}

func TestRegisterHandler_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockEmailSender := &MockEmailSender{}
	handler := &RegisterHandler{
		DB:          nil, // Not needed for this test
		EmailSender: mockEmailSender,
	}

	router := gin.New()
	router.POST("/auth/register", handler.Register)

	// Invalid JSON
	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid request format")
}

func TestRegisterHandler_InvalidPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockEmailSender := &MockEmailSender{}
	handler := &RegisterHandler{
		DB:          nil, // Not needed for this test
		EmailSender: mockEmailSender,
	}

	router := gin.New()
	router.POST("/auth/register", handler.Register)

	reqBody := RegisterRequest{
		Email:       "test@example.com",
		Password:    "weak",
		AccountName: "Test Account",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "password must be at least 12 characters long")
}

func TestRegisterHandler_CommonPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockEmailSender := &MockEmailSender{}
	handler := &RegisterHandler{
		DB:          nil, // Not needed for this test
		EmailSender: mockEmailSender,
	}

	router := gin.New()
	router.POST("/auth/register", handler.Register)

	reqBody := RegisterRequest{
		Email:       "test@example.com",
		Password:    "Password123!",
		AccountName: "Test Account",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "password is too common")
}

func TestRegisterHandler_DuplicateEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mockEmailSender := &MockEmailSender{}
	handler := &RegisterHandler{
		DB:          db,
		EmailSender: mockEmailSender,
	}

	router := gin.New()
	router.POST("/auth/register", handler.Register)

	reqBody := RegisterRequest{
		Email:       "existing@example.com",
		Password:    "SecurePass123!",
		AccountName: "Test Account",
	}
	jsonBody, _ := json.Marshal(reqBody)

	// Mock: email already exists
	mock.ExpectQuery("SELECT id FROM users WHERE email").
		WithArgs(reqBody.Email).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("existing-user-id"))

	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "Email already registered")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRegisterHandler_VerificationTokenIsUnique(t *testing.T) {
	// Verify that generateSecureToken produces unique tokens
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := generateSecureToken(32)
		assert.NoError(t, err)
		assert.NotEmpty(t, token)
		assert.False(t, tokens[token], "Token collision detected")
		tokens[token] = true
	}
}

// Integration test - requires database connection
// This test is skipped by default and should be run with -tags=integration
func TestRegisterHandler_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// This would require a test database setup
	// For now, we'll skip this test
	t.Skip("Integration test requires database setup")
}
