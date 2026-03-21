package middleware

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ResourceType identifies which plan limit to check.
type ResourceType string

const (
	ResourceCloudAccount       ResourceType = "cloud_account"
	ResourceDatabaseConnection ResourceType = "database_connection"
)

// PlanLimits holds the limits and current usage for an account's subscription plan.
type PlanLimits struct {
	PlanName                   string `json:"plan_name"`
	MaxCloudAccounts           *int   `json:"max_cloud_accounts"`       // nil = unlimited
	MaxDatabaseConnections     *int   `json:"max_database_connections"` // nil = unlimited
	CurrentCloudAccounts       int    `json:"current_cloud_accounts"`
	CurrentDatabaseConnections int    `json:"current_database_connections"`
}

// getAccountPlanLimits queries the active subscription plan and current usage for an account.
func getAccountPlanLimits(db *sql.DB, accountID string) (*PlanLimits, error) {
	var limits PlanLimits

	// Query active subscription plan limits
	err := db.QueryRow(`
		SELECT sp.name, sp.max_cloud_accounts, sp.max_database_connections
		FROM stripe_subscriptions ss
		JOIN subscription_plans sp ON ss.plan_id = sp.id
		WHERE ss.account_id = ?
		  AND ss.status IN ('active', 'trialing')
		  AND ss.deleted_at IS NULL
		ORDER BY ss.created_at DESC
		LIMIT 1`,
		accountID,
	).Scan(&limits.PlanName, &limits.MaxCloudAccounts, &limits.MaxDatabaseConnections)

	if err == sql.ErrNoRows {
		// Default to free plan limits if no subscription found
		free := 1
		freeDB := 2
		limits.PlanName = "free"
		limits.MaxCloudAccounts = &free
		limits.MaxDatabaseConnections = &freeDB
	} else if err != nil {
		return nil, fmt.Errorf("failed to query subscription plan: %w", err)
	}

	// Count current cloud accounts (non-deleted)
	err = db.QueryRow(`
		SELECT COUNT(*) FROM cloud_accounts
		WHERE account_id = ? AND deleted_at IS NULL`,
		accountID,
	).Scan(&limits.CurrentCloudAccounts)
	if err != nil {
		return nil, fmt.Errorf("failed to count cloud accounts: %w", err)
	}

	// Count current database connections (non-deleted)
	err = db.QueryRow(`
		SELECT COUNT(*) FROM database_connections
		WHERE account_id = ? AND deleted_at IS NULL`,
		accountID,
	).Scan(&limits.CurrentDatabaseConnections)
	if err != nil {
		return nil, fmt.Errorf("failed to count database connections: %w", err)
	}

	return &limits, nil
}

// CheckPlanLimit returns a Gin middleware that enforces the plan limit for the given resource type.
// It expects account_id to be set in the Gin context (by auth middleware) under the key "account_id".
func CheckPlanLimit(db *sql.DB, resourceType ResourceType) gin.HandlerFunc {
	return func(c *gin.Context) {
		accountID, exists := c.Get("account_id")
		if !exists || accountID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "account_id not found in request context"})
			c.Abort()
			return
		}

		limits, err := getAccountPlanLimits(db, fmt.Sprintf("%v", accountID))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check plan limits"})
			c.Abort()
			return
		}

		switch resourceType {
		case ResourceCloudAccount:
			if limits.MaxCloudAccounts != nil && limits.CurrentCloudAccounts >= *limits.MaxCloudAccounts {
				c.JSON(http.StatusForbidden, gin.H{
					"error": fmt.Sprintf(
						"Cloud account limit reached (%d/%d). Please upgrade your plan to add more cloud accounts.",
						limits.CurrentCloudAccounts, *limits.MaxCloudAccounts,
					),
					"limit_exceeded":   true,
					"resource_type":    string(ResourceCloudAccount),
					"current_usage":    limits.CurrentCloudAccounts,
					"plan_limit":       *limits.MaxCloudAccounts,
					"current_plan":     limits.PlanName,
					"upgrade_required": true,
				})
				c.Abort()
				return
			}

		case ResourceDatabaseConnection:
			if limits.MaxDatabaseConnections != nil && limits.CurrentDatabaseConnections >= *limits.MaxDatabaseConnections {
				c.JSON(http.StatusForbidden, gin.H{
					"error": fmt.Sprintf(
						"Database connection limit reached (%d/%d). Please upgrade your plan to add more database connections.",
						limits.CurrentDatabaseConnections, *limits.MaxDatabaseConnections,
					),
					"limit_exceeded":   true,
					"resource_type":    string(ResourceDatabaseConnection),
					"current_usage":    limits.CurrentDatabaseConnections,
					"plan_limit":       *limits.MaxDatabaseConnections,
					"current_plan":     limits.PlanName,
					"upgrade_required": true,
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}
