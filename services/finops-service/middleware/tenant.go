package middleware

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TenantMiddleware enforces multi-tenant data isolation.
//
// For regular users it validates that account_id is present in the Gin context
// (set by the auth middleware) and passes it through unchanged.
//
// For super_admin users it allows an optional ?account_id=<uuid> query parameter
// to impersonate any account for support purposes. When a super_admin accesses a
// different account the access is recorded in audit_logs.
func TenantMiddleware(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// account_id must have been set by the upstream auth middleware.
		accountID := c.GetString("account_id")
		if accountID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing account context"})
			c.Abort()
			return
		}

		// Check whether the caller is a super_admin.
		if isSuperAdmin(c) {
			// Allow the caller to target a different account via query param.
			if targetAccountID := c.Query("account_id"); targetAccountID != "" && targetAccountID != accountID {
				// Log the cross-account access before overriding the context value.
				logSuperAdminAccess(db, c, accountID, targetAccountID)
				c.Set("account_id", targetAccountID)
			}
			// Super_Admin always proceeds.
			c.Next()
			return
		}

		// Regular users: account_id is already set correctly by auth middleware.
		c.Next()
	}
}

// isSuperAdmin returns true when the "user_role" or "roles" context value
// indicates the caller holds the super_admin role.
func isSuperAdmin(c *gin.Context) bool {
	// Check "user_role" (single string, set by some middleware variants).
	if role := c.GetString("user_role"); role == "super_admin" {
		return true
	}

	// Check "roles" ([]string, set by the JWT/API-key auth middleware).
	if rolesVal, exists := c.Get("roles"); exists {
		if roles, ok := rolesVal.([]string); ok {
			for _, r := range roles {
				if r == "super_admin" {
					return true
				}
			}
		}
	}

	return false
}

// logSuperAdminAccess inserts an audit_log entry for a super_admin cross-account access.
func logSuperAdminAccess(db *sql.DB, c *gin.Context, superAdminAccountID, targetAccountID string) {
	if db == nil {
		return
	}
	userID := c.GetString("user_id")
	id := uuid.New().String()
	_, _ = db.Exec(
		`INSERT INTO audit_logs
		 (id, user_id, account_id, action_type, resource_type, resource_id, ip_address, user_agent, created_at)
		 VALUES (?, ?, ?, 'super_admin_access', 'account', ?, ?, ?, NOW())`,
		id, userID, superAdminAccountID, targetAccountID,
		c.ClientIP(), c.Request.UserAgent(),
	)
}
