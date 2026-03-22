package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type RegisterRequest struct {
	Email         string `json:"email" binding:"required,email"`
	Password      string `json:"password" binding:"required"`
	AccountName   string `json:"account_name" binding:"required"`
	TermsAccepted bool   `json:"terms_accepted"`
}

type RegisterResponse struct {
	UserID           string `json:"user_id"`
	VerificationSent bool   `json:"verification_sent"`
}

type EmailSender interface {
	SendVerificationEmail(toEmail, verificationToken string) error
	SendOTPEmail(toEmail, otp string) error
}

type RegisterHandler struct {
	DB          *sql.DB
	EmailSender EmailSender
}

// Register handles user registration with email verification
func (h *RegisterHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate password requirements
	if err := validatePassword(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Terms must be accepted
	if !req.TermsAccepted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You must accept the terms and conditions"})
		return
	}

	// Check if email already exists
	var existingUserID string
	err := h.DB.QueryRow("SELECT id FROM users WHERE email = ? AND deleted_at IS NULL", req.Email).Scan(&existingUserID)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	} else if err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Hash password with bcrypt cost factor 12
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Generate verification token
	verificationToken, err := generateSecureToken(32)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate verification token"})
		return
	}

	// Token expires in 24 hours
	tokenExpiry := time.Now().Add(24 * time.Hour)

	// Start transaction
	tx, err := h.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer tx.Rollback()

	// Create account
	accountID := uuid.New().String()
	_, err = tx.Exec(
		"INSERT INTO accounts (id, name, created_at, updated_at) VALUES (?, ?, NOW(), NOW())",
		accountID, req.AccountName,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account"})
		return
	}

	// Create user
	userID := uuid.New().String()
	_, err = tx.Exec(
		`INSERT INTO users (id, account_id, email, password_hash, email_verified,
		verification_token, verification_token_expiry, last_password_change,
		terms_accepted, terms_accepted_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, FALSE, ?, ?, NOW(), TRUE, NOW(), NOW(), NOW())`,
		userID, accountID, req.Email, string(passwordHash), verificationToken, tokenExpiry,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Seed a free subscription for the new account so billing page loads immediately
	var freePlanID string
	freePlanErr := h.DB.QueryRow(`SELECT id FROM subscription_plans WHERE name='free' LIMIT 1`).Scan(&freePlanID)
	if freePlanErr == nil && freePlanID != "" {
		subID := uuid.New().String()
		_, _ = tx.Exec(
			`INSERT INTO stripe_subscriptions
			 (id, account_id, stripe_subscription_id, plan_id, status, current_period_start, current_period_end, created_at, updated_at)
			 VALUES (?, ?, ?, ?, 'active', NOW(), DATE_ADD(NOW(), INTERVAL 30 DAY), NOW(), NOW())`,
			subID, accountID, "free_"+accountID, freePlanID,
		)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete registration"})
		return
	}

	// Send verification email
	verificationSent := false
	if h.EmailSender != nil {
		if err := h.EmailSender.SendVerificationEmail(req.Email, verificationToken); err == nil {
			verificationSent = true
		}
	}

	c.JSON(http.StatusCreated, RegisterResponse{
		UserID:           userID,
		VerificationSent: verificationSent,
	})
}

// validatePassword checks password requirements
func validatePassword(password string) error {
	if len(password) < 12 {
		return fmt.Errorf("password must be at least 12 characters long")
	}

	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasNumber := regexp.MustCompile(`[0-9]`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password)

	if !hasUpper {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}
	if !hasNumber {
		return fmt.Errorf("password must contain at least one number")
	}
	if !hasSpecial {
		return fmt.Errorf("password must contain at least one special character")
	}

	// Check against common passwords
	if isCommonPassword(password) {
		return fmt.Errorf("password is too common, please choose a stronger password")
	}

	return nil
}

// isCommonPassword checks if password is in top 10,000 common passwords
func isCommonPassword(password string) bool {
	lowerPassword := strings.ToLower(password)

	// Check both original and lowercase versions
	return commonPasswordsList[password] || commonPasswordsList[lowerPassword]
}

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
