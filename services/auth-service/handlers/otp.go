package handlers

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/finops-platform/auth-service/utils"
	"github.com/gin-gonic/gin"
)

// OTPHandler handles OTP-based email verification
type OTPHandler struct {
	DB          *sql.DB
	EmailSender EmailSender
	JWTService  *utils.JWTService
}

type sendOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type verifyOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
	OTP   string `json:"otp" binding:"required"`
}

func generateOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// SendOTP generates and emails a 6-digit OTP to the user.
// POST /auth/send-otp
func (h *OTPHandler) SendOTP(c *gin.Context) {
	var req sendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid email is required"})
		return
	}

	var userID string
	err := h.DB.QueryRow(
		`SELECT id FROM users WHERE email = ? AND deleted_at IS NULL`, req.Email,
	).Scan(&userID)
	if err == sql.ErrNoRows {
		// Don't reveal whether email exists
		c.JSON(http.StatusOK, gin.H{"sent": true})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	otp, err := generateOTP()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate OTP"})
		return
	}

	expiry := time.Now().Add(10 * time.Minute)
	_, err = h.DB.Exec(
		`UPDATE users SET verification_token = ?, verification_token_expiry = ?, updated_at = NOW() WHERE id = ?`,
		otp, expiry, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store OTP"})
		return
	}

	if h.EmailSender != nil {
		_ = h.EmailSender.SendOTPEmail(req.Email, otp)
	}

	c.JSON(http.StatusOK, gin.H{"sent": true})
}

// VerifyOTP checks the OTP, marks email verified, and returns JWT tokens.
// POST /auth/verify-otp
func (h *OTPHandler) VerifyOTP(c *gin.Context) {
	var req verifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and otp are required"})
		return
	}

	var userID, accountID, storedOTP string
	var expiry time.Time

	err := h.DB.QueryRow(
		`SELECT id, account_id, COALESCE(verification_token,''), COALESCE(verification_token_expiry, NOW())
		 FROM users WHERE email = ? AND deleted_at IS NULL`,
		req.Email,
	).Scan(&userID, &accountID, &storedOTP, &expiry)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid code"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	if storedOTP != req.OTP {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid code"})
		return
	}
	if time.Now().After(expiry) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "code expired, request a new one"})
		return
	}

	_, err = h.DB.Exec(
		`UPDATE users SET email_verified = TRUE, verification_token = NULL,
		 verification_token_expiry = NULL, updated_at = NOW() WHERE id = ?`,
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify"})
		return
	}

	roles, _ := fetchUserRoles(h.DB, userID)

	accessToken, err := h.JWTService.GenerateAccessToken(userID, accountID, roles)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	refreshToken, err := h.JWTService.GenerateRefreshToken(userID, accountID, roles)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	utils.LogAuditEvent(h.DB, utils.AuditEvent{
		UserID: userID, AccountID: accountID,
		ActionType: "otp_verified", ResourceType: "user", ResourceID: userID,
		IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(),
	})

	c.SetCookie("refresh_token", refreshToken, 7*24*3600, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    900,
	})
}
