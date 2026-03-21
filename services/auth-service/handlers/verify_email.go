package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type VerifyEmailRequest struct {
	Token string `form:"token" binding:"required"`
}

type VerifyEmailResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type VerifyEmailHandler struct {
	DB *sql.DB
}

// VerifyEmail handles email verification via token
func (h *VerifyEmailHandler) VerifyEmail(c *gin.Context) {
	var req VerifyEmailRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Verification token is required",
		})
		return
	}

	// Query user by verification token
	var userID string
	var emailVerified bool
	var tokenExpiry time.Time

	err := h.DB.QueryRow(
		`SELECT id, email_verified, verification_token_expiry 
		FROM users 
		WHERE verification_token = ? AND deleted_at IS NULL`,
		req.Token,
	).Scan(&userID, &emailVerified, &tokenExpiry)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Invalid verification token",
		})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database error",
		})
		return
	}

	// Check if email is already verified
	if emailVerified {
		c.JSON(http.StatusOK, VerifyEmailResponse{
			Success: true,
			Message: "Email already verified",
		})
		return
	}

	// Check if token has expired
	if time.Now().After(tokenExpiry) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Verification token has expired",
		})
		return
	}

	// Update user: set email_verified to true and clear verification token
	_, err = h.DB.Exec(
		`UPDATE users 
		SET email_verified = TRUE, 
		    verification_token = NULL, 
		    verification_token_expiry = NULL,
		    updated_at = NOW()
		WHERE id = ?`,
		userID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to verify email",
		})
		return
	}

	c.JSON(http.StatusOK, VerifyEmailResponse{
		Success: true,
		Message: "Email verified successfully",
	})
}
