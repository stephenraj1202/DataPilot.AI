package handlers

import (
	"database/sql"
	"net/http"

	"github.com/finops-platform/auth-service/utils"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// LoginHandler handles user login requests
type LoginHandler struct {
	DB         *sql.DB
	JWTService *utils.JWTService
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // seconds
}

// Login authenticates a user and returns JWT tokens
func (h *LoginHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Fetch user from database
	var userID, accountID, passwordHash string
	var emailVerified bool
	var fullName, avatarURL string
	err := h.DB.QueryRow(
		`SELECT id, account_id, password_hash, email_verified,
		        COALESCE(full_name,''), COALESCE(avatar_url,'')
		 FROM users WHERE email = ? AND deleted_at IS NULL`,
		req.Email,
	).Scan(&userID, &accountID, &passwordHash, &emailVerified, &fullName, &avatarURL)
	if err == sql.ErrNoRows {
		utils.LogAuditEvent(h.DB, utils.AuditEvent{
			ActionType: "login_failed", ResourceType: "user",
			IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(),
		})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		utils.LogAuditEvent(h.DB, utils.AuditEvent{
			UserID: userID, AccountID: accountID,
			ActionType: "login_failed", ResourceType: "user", ResourceID: userID,
			IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(),
		})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	// Check email verification
	if !emailVerified {
		c.JSON(http.StatusForbidden, gin.H{"error": "email not verified, please check your inbox"})
		return
	}

	// Fetch user roles
	roles, err := fetchUserRoles(h.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// Generate tokens
	profile := utils.UserProfile{Name: fullName, Avatar: avatarURL, Email: req.Email}
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

	// Log successful authentication
	utils.LogAuditEvent(h.DB, utils.AuditEvent{
		UserID: userID, AccountID: accountID,
		ActionType: "login_success", ResourceType: "user", ResourceID: userID,
		IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(),
	})

	// Set refresh token as httpOnly cookie
	c.SetCookie("refresh_token", refreshToken, 7*24*3600, "/", "", false, true)

	c.JSON(http.StatusOK, loginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900,
	})
}

// fetchUserRoles retrieves role names for a given user
func fetchUserRoles(db *sql.DB, userID string) ([]string, error) {
	rows, err := db.Query(
		`SELECT r.name FROM roles r
		 JOIN user_roles ur ON ur.role_id = r.id
		 WHERE ur.user_id = ?`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}
