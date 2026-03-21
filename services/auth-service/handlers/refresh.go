package handlers

import (
	"database/sql"
	"net/http"

	"github.com/finops-platform/auth-service/utils"
	"github.com/gin-gonic/gin"
)

// RefreshHandler handles token refresh requests
type RefreshHandler struct {
	DB         *sql.DB
	JWTService *utils.JWTService
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type refreshResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"` // seconds
}

// Refresh validates a refresh token and issues a new access token
func (h *RefreshHandler) Refresh(c *gin.Context) {
	// Prefer httpOnly cookie; fall back to JSON body
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		var req refreshRequest
		if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token required"})
			return
		}
		refreshToken = req.RefreshToken
	}

	// Validate the refresh token
	claims, err := h.JWTService.ValidateRefreshToken(refreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
		return
	}

	// Re-fetch profile from DB so the new access token always has current name/avatar
	var fullName, avatarURL, email string
	_ = h.DB.QueryRow(
		`SELECT COALESCE(full_name,''), COALESCE(avatar_url,''), COALESCE(email,'')
		 FROM users WHERE id = ? AND deleted_at IS NULL`,
		claims.UserID,
	).Scan(&fullName, &avatarURL, &email)

	// Fall back to whatever was in the refresh token if DB returned nothing
	if fullName == "" {
		fullName = claims.Name
	}
	if avatarURL == "" {
		avatarURL = claims.Avatar
	}
	if email == "" {
		email = claims.Email
	}

	profile := utils.UserProfile{Name: fullName, Avatar: avatarURL, Email: email}

	// Issue a new access token with fresh profile data
	accessToken, err := h.JWTService.GenerateAccessTokenWithProfile(claims.UserID, claims.AccountID, claims.Roles, profile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate access token"})
		return
	}

	c.JSON(http.StatusOK, refreshResponse{
		AccessToken: accessToken,
		ExpiresIn:   900,
	})
}
