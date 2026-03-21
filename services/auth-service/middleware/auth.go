package middleware

import (
	"net/http"
	"strings"

	"github.com/finops-platform/auth-service/utils"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates JWT tokens from the Authorization header
type AuthMiddleware struct {
	JWTService *utils.JWTService
}

// ValidateToken is a Gin middleware that validates JWT access tokens
func (m *AuthMiddleware) ValidateToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "missing authorization header",
			})
			c.Abort()
			return
		}

		// Check for Bearer token format
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid authorization header format, expected 'Bearer <token>'",
			})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Validate the access token
		claims, err := m.JWTService.ValidateAccessToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
			})
			c.Abort()
			return
		}

		// Attach claims to context for use in handlers
		c.Set("user_id", claims.UserID)
		c.Set("account_id", claims.AccountID)
		c.Set("roles", claims.Roles)

		c.Next()
	}
}

// GetUserID retrieves the user ID from the Gin context
func GetUserID(c *gin.Context) (string, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", false
	}
	return userID.(string), true
}

// GetAccountID retrieves the account ID from the Gin context
func GetAccountID(c *gin.Context) (string, bool) {
	accountID, exists := c.Get("account_id")
	if !exists {
		return "", false
	}
	return accountID.(string), true
}

// GetRoles retrieves the roles from the Gin context
func GetRoles(c *gin.Context) ([]string, bool) {
	roles, exists := c.Get("roles")
	if !exists {
		return nil, false
	}
	return roles.([]string), true
}
