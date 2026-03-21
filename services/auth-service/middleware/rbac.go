package middleware

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

const superAdminRoleID = "00000000-0000-0000-0000-000000000001"

// HasPermission checks whether the given user has the specified permission.
// Super_Admin users bypass all permission checks and always return true.
// The permission name follows the pattern "resource:action" (e.g. "finops:read").
func HasPermission(db *sql.DB, userID string, permission string) (bool, error) {
	// First check if the user has the super_admin role — bypass all checks.
	var superAdminCount int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM user_roles WHERE user_id = ? AND role_id = ?`,
		userID, superAdminRoleID,
	).Scan(&superAdminCount)
	if err != nil {
		return false, err
	}
	if superAdminCount > 0 {
		return true, nil
	}

	// Check whether any of the user's roles grant the requested permission.
	var count int
	err = db.QueryRow(
		`SELECT COUNT(*)
		 FROM user_roles ur
		 JOIN role_permissions rp ON rp.role_id = ur.role_id
		 JOIN permissions p       ON p.id = rp.permission_id
		 WHERE ur.user_id = ? AND p.name = ?`,
		userID, permission,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// RequirePermission returns a Gin middleware that enforces the given permission.
// It expects the auth middleware to have already set "user_id" in the context.
// Returns HTTP 403 Forbidden when the user lacks the required permission.
func RequirePermission(db *sql.DB, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user identity"})
			c.Abort()
			return
		}

		uid, ok := userID.(string)
		if !ok || uid == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user identity"})
			c.Abort()
			return
		}

		allowed, err := HasPermission(db, uid, permission)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "permission check failed"})
			c.Abort()
			return
		}

		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden: insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}
