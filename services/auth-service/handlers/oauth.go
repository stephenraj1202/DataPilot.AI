package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/finops-platform/auth-service/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// OAuthHandler handles Google SSO login/register
type OAuthHandler struct {
	DB         *sql.DB
	JWTService *utils.JWTService
}

type googleAuthRequest struct {
	// credential is the Google ID token from the frontend (Google One Tap / Sign-In button)
	Credential    string `json:"credential" binding:"required"`
	AccountName   string `json:"account_name"` // only needed when creating a new account
	TermsAccepted bool   `json:"terms_accepted"`
}

// googleTokenInfo is the response from Google's tokeninfo endpoint
type googleTokenInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Error         string `json:"error"`
}

func verifyGoogleToken(idToken string) (*googleTokenInfo, error) {
	url := "https://oauth2.googleapis.com/tokeninfo?id_token=" + idToken
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(url)
	if err != nil {
		return nil, fmt.Errorf("google token verification request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var info googleTokenInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse google token response: %w", err)
	}
	if info.Error != "" {
		return nil, fmt.Errorf("invalid google token: %s", info.Error)
	}
	if info.Email == "" {
		return nil, fmt.Errorf("google token missing email")
	}
	return &info, nil
}

// GoogleAuth handles POST /auth/google
// - Verifies the Google ID token
// - If user exists → log in
// - If user doesn't exist → create account + user (SSO register)
func (h *OAuthHandler) GoogleAuth(c *gin.Context) {
	var req googleAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "credential is required"})
		return
	}

	// Verify Google ID token
	info, err := verifyGoogleToken(req.Credential)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Look up existing user by email or oauth_provider_id
	var userID, accountID string
	var roles []string
	err = h.DB.QueryRow(
		`SELECT id, account_id FROM users WHERE email = ? AND deleted_at IS NULL`,
		info.Email,
	).Scan(&userID, &accountID)

	if err == sql.ErrNoRows {
		// New user — create account + user
		// For SSO registration, terms must be accepted
		if !req.TermsAccepted {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":    "terms_required",
				"message":  "Please accept the terms and conditions to create your account",
				"new_user": true,
				"email":    info.Email,
				"name":     info.Name,
				"picture":  info.Picture,
			})
			return
		}

		accountName := req.AccountName
		if accountName == "" {
			accountName = info.Name + "'s Organization"
		}

		tx, err := h.DB.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer tx.Rollback()

		accountID = uuid.New().String()
		_, err = tx.Exec(
			"INSERT INTO accounts (id, name, created_at, updated_at) VALUES (?, ?, NOW(), NOW())",
			accountID, accountName,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create account"})
			return
		}

		userID = uuid.New().String()
		now := time.Now()
		_, err = tx.Exec(
			`INSERT INTO users
			 (id, account_id, email, password_hash, email_verified, full_name, avatar_url,
			  oauth_provider, oauth_provider_id, terms_accepted, terms_accepted_at, created_at, updated_at)
			 VALUES (?, ?, ?, '', TRUE, ?, ?, 'google', ?, TRUE, ?, ?, ?)`,
			userID, accountID, info.Email, info.Name, info.Picture,
			info.Sub, now, now, now,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
			return
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to complete registration"})
			return
		}

		// Auto-create 30-day free trial subscription
		go createFreeTrialSubscription(h.DB, accountID)

		utils.LogAuditEvent(h.DB, utils.AuditEvent{
			UserID: userID, AccountID: accountID,
			ActionType: "oauth_register", ResourceType: "user", ResourceID: userID,
			IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(),
		})
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	} else {
		// Existing user — always sync latest name/avatar from Google
		_, _ = h.DB.Exec(
			`UPDATE users SET
			 oauth_provider    = COALESCE(oauth_provider, 'google'),
			 oauth_provider_id = COALESCE(oauth_provider_id, ?),
			 avatar_url        = IF(? != '', ?, avatar_url),
			 full_name         = IF(? != '' AND (full_name IS NULL OR full_name = ''), ?, full_name),
			 updated_at        = NOW()
			 WHERE id = ?`,
			info.Sub,
			info.Picture, info.Picture,
			info.Name, info.Name,
			userID,
		)

		utils.LogAuditEvent(h.DB, utils.AuditEvent{
			UserID: userID, AccountID: accountID,
			ActionType: "oauth_login", ResourceType: "user", ResourceID: userID,
			IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(),
		})
	}

	// Fetch roles
	roles, _ = fetchUserRoles(h.DB, userID)

	// Fetch full_name and avatar_url for the token
	var fullName, avatarURL string
	_ = h.DB.QueryRow(`SELECT COALESCE(full_name,''), COALESCE(avatar_url,'') FROM users WHERE id=?`, userID).
		Scan(&fullName, &avatarURL)

	profile := utils.UserProfile{Name: fullName, Avatar: avatarURL, Email: info.Email}

	// Generate tokens
	accessToken, err := h.JWTService.GenerateAccessTokenWithProfile(userID, accountID, roles, profile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate access token"})
		return
	}
	refreshToken, err := h.JWTService.GenerateRefreshTokenWithProfile(userID, accountID, roles, profile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}

	c.SetCookie("refresh_token", refreshToken, 7*24*3600, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    900,
		"new_user":      false,
	})
}

// createFreeTrialSubscription inserts a 30-day free trial subscription for a new account.
func createFreeTrialSubscription(db *sql.DB, accountID string) {
	var planID string
	err := db.QueryRow(`SELECT id FROM subscription_plans WHERE name = 'free' LIMIT 1`).Scan(&planID)
	if err != nil {
		return
	}
	subID := uuid.New().String()
	_, _ = db.Exec(
		`INSERT IGNORE INTO stripe_subscriptions
		 (id, account_id, stripe_subscription_id, plan_id, status, current_period_start, current_period_end)
		 VALUES (?, ?, ?, ?, 'trialing', NOW(), DATE_ADD(NOW(), INTERVAL 30 DAY))`,
		subID, accountID, "trial_"+accountID, planID,
	)
}
