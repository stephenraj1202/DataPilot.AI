package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

var errPasswordReused = errors.New("password has been used recently, please choose a different password")

// PasswordResetHandler handles forgot-password and reset-password flows
type PasswordResetHandler struct {
	DB          *sql.DB
	EmailSender EmailSender
}

type forgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type resetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// PasswordResetEmailSender is an optional interface for sending password reset emails
type PasswordResetEmailSender interface {
	SendPasswordResetEmail(toEmail, resetToken string) error
}

// ForgotPassword generates a password reset token and sends it via email
func (h *PasswordResetHandler) ForgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Look up user by email (don't reveal whether email exists)
	var userID string
	err := h.DB.QueryRow(
		`SELECT id FROM users WHERE email = ? AND deleted_at IS NULL`,
		req.Email,
	).Scan(&userID)
	if err == sql.ErrNoRows {
		// Return success to prevent email enumeration
		c.JSON(http.StatusOK, gin.H{"message": "If that email is registered, a reset link has been sent"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Generate reset token
	resetToken, err := generateSecureToken(32)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate reset token"})
		return
	}

	tokenExpiry := time.Now().Add(1 * time.Hour)

	_, err = h.DB.Exec(
		`UPDATE users SET reset_token = ?, reset_token_expiry = ?, updated_at = NOW() WHERE id = ?`,
		resetToken, tokenExpiry, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store reset token"})
		return
	}

	// Send reset email (best-effort)
	if h.EmailSender != nil {
		if sender, ok := h.EmailSender.(PasswordResetEmailSender); ok {
			_ = sender.SendPasswordResetEmail(req.Email, resetToken)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "If that email is registered, a reset link has been sent"})
}

// ResetPassword validates the reset token and updates the password
func (h *PasswordResetHandler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate new password
	if err := validatePassword(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Look up user by reset token
	var userID string
	var tokenExpiry time.Time
	var passwordHistoryJSON sql.NullString
	var currentHash string

	err := h.DB.QueryRow(
		`SELECT id, reset_token_expiry, password_history, password_hash
		 FROM users WHERE reset_token = ? AND deleted_at IS NULL`,
		req.Token,
	).Scan(&userID, &tokenExpiry, &passwordHistoryJSON, &currentHash)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset token"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Check token expiry
	if time.Now().After(tokenExpiry) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset token"})
		return
	}

	// Check password history (last 5 passwords)
	if err := checkPasswordHistory(req.Password, currentHash, passwordHistoryJSON.String); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Hash new password
	newHash, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Build updated password history (prepend current hash, keep last 5)
	updatedHistory, err := buildPasswordHistory(currentHash, passwordHistoryJSON.String)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password history"})
		return
	}

	_, err = h.DB.Exec(
		`UPDATE users
		 SET password_hash = ?, reset_token = NULL, reset_token_expiry = NULL,
		     last_password_change = NOW(), password_history = ?, updated_at = NOW()
		 WHERE id = ?`,
		newHash, updatedHistory, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}

// hashPassword hashes a password using bcrypt with cost factor 12
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// comparePasswordHash compares a bcrypt hash with a plaintext password
func comparePasswordHash(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// checkPasswordHistory returns an error if the new password matches any of the
// last 5 stored hashes (including the current password).
func checkPasswordHistory(newPassword, currentHash, historyJSON string) error {
	if comparePasswordHash(currentHash, newPassword) == nil {
		return errPasswordReused
	}

	if historyJSON == "" {
		return nil
	}

	var history []string
	if err := json.Unmarshal([]byte(historyJSON), &history); err != nil {
		return nil // ignore malformed history
	}

	limit := 5
	if len(history) < limit {
		limit = len(history)
	}
	for _, h := range history[:limit] {
		if comparePasswordHash(h, newPassword) == nil {
			return errPasswordReused
		}
	}
	return nil
}

// buildPasswordHistory prepends currentHash to the existing history and keeps at most 5 entries
func buildPasswordHistory(currentHash, historyJSON string) (string, error) {
	var history []string
	if historyJSON != "" {
		if err := json.Unmarshal([]byte(historyJSON), &history); err != nil {
			history = []string{}
		}
	}

	// Prepend current hash
	history = append([]string{currentHash}, history...)

	// Keep only last 5
	if len(history) > 5 {
		history = history[:5]
	}

	data, err := json.Marshal(history)
	if err != nil {
		return "", fmt.Errorf("failed to marshal password history: %w", err)
	}
	return string(data), nil
}
