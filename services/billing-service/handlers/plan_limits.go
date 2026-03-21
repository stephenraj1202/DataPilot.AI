package handlers

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// PlanLimitsHandler handles plan limit queries.
type PlanLimitsHandler struct {
	DB *sql.DB
}

// planLimitsResponse is the response body for GET /billing/plan-limits.
type planLimitsResponse struct {
	PlanName                     string `json:"plan_name"`
	MaxCloudAccounts             *int   `json:"max_cloud_accounts"`       // nil = unlimited
	MaxDatabaseConnections       *int   `json:"max_database_connections"` // nil = unlimited
	CurrentCloudAccounts         int    `json:"current_cloud_accounts"`
	CurrentDatabaseConnections   int    `json:"current_database_connections"`
	CloudAccountsRemaining       *int   `json:"cloud_accounts_remaining"`       // nil = unlimited
	DatabaseConnectionsRemaining *int   `json:"database_connections_remaining"` // nil = unlimited
}

// GetPlanLimits returns the current plan limits and usage for an account.
// GET /billing/plan-limits
// Requires account_id in the Gin context (set by auth middleware).
func (h *PlanLimitsHandler) GetPlanLimits(c *gin.Context) {
	accountID, exists := c.Get("account_id")
	if !exists || accountID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "account_id not found in request context"})
		return
	}

	resp, err := buildPlanLimitsResponse(h.DB, fmt.Sprintf("%v", accountID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve plan limits"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// buildPlanLimitsResponse queries the DB and constructs the plan limits response.
func buildPlanLimitsResponse(db *sql.DB, accountID string) (*planLimitsResponse, error) {
	resp := &planLimitsResponse{}

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
	).Scan(&resp.PlanName, &resp.MaxCloudAccounts, &resp.MaxDatabaseConnections)

	if err == sql.ErrNoRows {
		// Default to free plan limits if no subscription found
		free := 1
		freeDB := 2
		resp.PlanName = "free"
		resp.MaxCloudAccounts = &free
		resp.MaxDatabaseConnections = &freeDB
	} else if err != nil {
		return nil, fmt.Errorf("failed to query subscription plan: %w", err)
	}

	// Count current cloud accounts (non-deleted)
	err = db.QueryRow(`
		SELECT COUNT(*) FROM cloud_accounts
		WHERE account_id = ? AND deleted_at IS NULL`,
		accountID,
	).Scan(&resp.CurrentCloudAccounts)
	if err != nil {
		return nil, fmt.Errorf("failed to count cloud accounts: %w", err)
	}

	// Count current database connections (non-deleted)
	err = db.QueryRow(`
		SELECT COUNT(*) FROM database_connections
		WHERE account_id = ? AND deleted_at IS NULL`,
		accountID,
	).Scan(&resp.CurrentDatabaseConnections)
	if err != nil {
		return nil, fmt.Errorf("failed to count database connections: %w", err)
	}

	// Calculate remaining capacity (nil = unlimited)
	if resp.MaxCloudAccounts != nil {
		remaining := *resp.MaxCloudAccounts - resp.CurrentCloudAccounts
		if remaining < 0 {
			remaining = 0
		}
		resp.CloudAccountsRemaining = &remaining
	}
	if resp.MaxDatabaseConnections != nil {
		remaining := *resp.MaxDatabaseConnections - resp.CurrentDatabaseConnections
		if remaining < 0 {
			remaining = 0
		}
		resp.DatabaseConnectionsRemaining = &remaining
	}

	return resp, nil
}
