package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// InjectContext reads the X-User-ID and X-Account-ID headers injected by the
// API Gateway and stores them in the Gin context so handlers can call
// c.GetString("user_id") and c.GetString("account_id").
func InjectContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		accountID := c.GetHeader("X-Account-ID")

		if userID != "" {
			c.Set("user_id", userID)
		}
		if accountID != "" {
			c.Set("account_id", accountID)
		}

		c.Next()
	}
}

// RequireAccount aborts with 401 when account_id is not present in context.
func RequireAccount() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("account_id") == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
