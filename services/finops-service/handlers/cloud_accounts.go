package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CloudAccountHandler handles cloud account CRUD operations.
type CloudAccountHandler struct {
	DB        *sql.DB
	AESKey    string
	EmailCfg  EmailConfig
	Scheduler *SyncScheduler
}

type connectCloudAccountRequest struct {
	Provider    string            `json:"provider" binding:"required"`
	AccountName string            `json:"account_name" binding:"required"`
	Credentials map[string]string `json:"credentials" binding:"required"`
}

// ConnectCloudAccount handles POST /finops/cloud-accounts
func (h *CloudAccountHandler) ConnectCloudAccount(c *gin.Context) {
	accountID := c.GetString("account_id")
	userID := c.GetString("user_id")

	var req connectCloudAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate provider
	provider, err := NewCloudProvider(req.Provider)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate credentials
	if err := provider.ValidateCredentials(req.Credentials); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Encrypt credentials
	credsJSON, err := json.Marshal(req.Credentials)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to serialize credentials"})
		return
	}

	encryptedCreds, err := encrypt(string(credsJSON), h.AESKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt credentials"})
		return
	}

	// Store in database
	id := uuid.New().String()
	now := time.Now()
	_, err = h.DB.Exec(
		`INSERT INTO cloud_accounts (id, account_id, provider, account_name, encrypted_credentials, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 'active', ?, ?)`,
		id, accountID, req.Provider, req.AccountName, encryptedCreds, now, now,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store cloud account"})
		return
	}

	logAuditEvent(h.DB, userID, accountID, "create", "cloud_account", id, c.ClientIP(), c.Request.UserAgent())

	c.JSON(http.StatusCreated, gin.H{
		"cloud_account_id": id,
		"status":           "connected",
		"last_sync":        nil,
	})
}

// ListCloudAccounts handles GET /finops/cloud-accounts
func (h *CloudAccountHandler) ListCloudAccounts(c *gin.Context) {
	accountID := c.GetString("account_id")

	rows, err := h.DB.Query(
		`SELECT id, provider, account_name, status, last_sync_at, last_sync_status, created_at
		 FROM cloud_accounts
		 WHERE account_id = ? AND deleted_at IS NULL
		 ORDER BY created_at DESC`,
		accountID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query cloud accounts"})
		return
	}
	defer rows.Close()

	type cloudAccountRow struct {
		ID             string  `json:"id"`
		Provider       string  `json:"provider"`
		AccountName    string  `json:"account_name"`
		Status         string  `json:"status"`
		LastSyncAt     *string `json:"last_sync_at"`
		LastSyncStatus *string `json:"last_sync_status"`
		CreatedAt      string  `json:"created_at"`
	}

	var accounts []cloudAccountRow
	for rows.Next() {
		var a cloudAccountRow
		if err := rows.Scan(&a.ID, &a.Provider, &a.AccountName, &a.Status,
			&a.LastSyncAt, &a.LastSyncStatus, &a.CreatedAt); err != nil {
			continue
		}
		accounts = append(accounts, a)
	}

	c.JSON(http.StatusOK, gin.H{"cloud_accounts": accounts})
}

// UpdateCloudAccount handles PUT /finops/cloud-accounts/:id
func (h *CloudAccountHandler) UpdateCloudAccount(c *gin.Context) {
	accountID := c.GetString("account_id")
	userID := c.GetString("user_id")
	cloudAccountID := c.Param("id")

	var req struct {
		AccountName string            `json:"account_name"`
		Credentials map[string]string `json:"credentials"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Fetch existing record to verify ownership
	var existingProvider string
	var existingEncCreds string
	err := h.DB.QueryRow(
		`SELECT provider, encrypted_credentials FROM cloud_accounts WHERE id = ? AND account_id = ? AND deleted_at IS NULL`,
		cloudAccountID, accountID,
	).Scan(&existingProvider, &existingEncCreds)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "cloud account not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	encCreds := existingEncCreds
	// If new credentials provided, validate and re-encrypt
	if len(req.Credentials) > 0 {
		provider, err := NewCloudProvider(existingProvider)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unknown provider"})
			return
		}
		if err := provider.ValidateCredentials(req.Credentials); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		credsJSON, _ := json.Marshal(req.Credentials)
		encCreds, err = encrypt(string(credsJSON), h.AESKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt credentials"})
			return
		}
	}

	// Build update query
	if req.AccountName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_name is required"})
		return
	}

	_, err = h.DB.Exec(
		`UPDATE cloud_accounts SET account_name = ?, encrypted_credentials = ?, updated_at = NOW() WHERE id = ? AND account_id = ?`,
		req.AccountName, encCreds, cloudAccountID, accountID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update cloud account"})
		return
	}

	logAuditEvent(h.DB, userID, accountID, "update", "cloud_account", cloudAccountID, c.ClientIP(), c.Request.UserAgent())
	c.JSON(http.StatusOK, gin.H{"message": "cloud account updated"})
}

// DeleteCloudAccount handles DELETE /finops/cloud-accounts/:id
func (h *CloudAccountHandler) DeleteCloudAccount(c *gin.Context) {
	accountID := c.GetString("account_id")
	userID := c.GetString("user_id")

	cloudAccountID := c.Param("id")
	result, err := h.DB.Exec(
		`UPDATE cloud_accounts SET deleted_at = NOW() WHERE id = ? AND account_id = ? AND deleted_at IS NULL`,
		cloudAccountID, accountID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete cloud account"})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "cloud account not found"})
		return
	}

	logAuditEvent(h.DB, userID, accountID, "delete", "cloud_account", cloudAccountID, c.ClientIP(), c.Request.UserAgent())
	c.JSON(http.StatusOK, gin.H{"message": "cloud account deleted"})
}

// GetCloudAccountResources handles GET /finops/cloud-accounts/:id/resources
// Returns service breakdown and region list from synced cost data.
func (h *CloudAccountHandler) GetCloudAccountResources(c *gin.Context) {
	accountID := c.GetString("account_id")
	cloudAccountID := c.Param("id")

	// Verify ownership
	var provider, accountName string
	var lastSyncAt *string
	err := h.DB.QueryRow(
		`SELECT provider, account_name, last_sync_at FROM cloud_accounts WHERE id = ? AND account_id = ? AND deleted_at IS NULL`,
		cloudAccountID, accountID,
	).Scan(&provider, &accountName, &lastSyncAt)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "cloud account not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	// MTD cost
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	today := now.Format("2006-01-02")

	var mtdCost float64
	h.DB.QueryRow(
		`SELECT COALESCE(SUM(cost_amount),0) FROM cloud_costs WHERE cloud_account_id = ? AND date BETWEEN ? AND ?`,
		cloudAccountID, monthStart, today,
	).Scan(&mtdCost)

	// Services with cost this month
	type serviceRow struct {
		Service string  `json:"service"`
		Cost    float64 `json:"cost"`
		Region  string  `json:"region"`
	}
	rows, err := h.DB.Query(
		`SELECT service_name, COALESCE(SUM(cost_amount),0) as total, COALESCE(region,'global') as region
		 FROM cloud_costs
		 WHERE cloud_account_id = ? AND date BETWEEN ? AND ?
		 GROUP BY service_name, region
		 ORDER BY total DESC`,
		cloudAccountID, monthStart, today,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query services"})
		return
	}
	defer rows.Close()

	var services []serviceRow
	regionSet := map[string]bool{}
	for rows.Next() {
		var s serviceRow
		if err := rows.Scan(&s.Service, &s.Cost, &s.Region); err == nil {
			services = append(services, s)
			regionSet[s.Region] = true
		}
	}

	regions := make([]string, 0, len(regionSet))
	for r := range regionSet {
		regions = append(regions, r)
	}

	c.JSON(http.StatusOK, gin.H{
		"id":            cloudAccountID,
		"provider":      provider,
		"account_name":  accountName,
		"last_sync_at":  lastSyncAt,
		"mtd_cost":      mtdCost,
		"service_count": len(services),
		"region_count":  len(regions),
		"services":      services,
		"regions":       regions,
	})
}

// GetCloudAccountVMResources handles GET /finops/cloud-accounts/:id/vm-resources
// Returns resource-type breakdown derived from cost data service names.
func (h *CloudAccountHandler) GetCloudAccountVMResources(c *gin.Context) {
	accountID := c.GetString("account_id")
	cloudAccountID := c.Param("id")

	var provider string
	err := h.DB.QueryRow(
		`SELECT provider FROM cloud_accounts WHERE id = ? AND account_id = ? AND deleted_at IS NULL`,
		cloudAccountID, accountID,
	).Scan(&provider)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "cloud account not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	today := now.Format("2006-01-02")

	// Fetch all distinct services with their costs and regions this month
	rows, err := h.DB.Query(
		`SELECT service_name, COALESCE(SUM(cost_amount),0), COALESCE(region,'global')
		 FROM cloud_costs
		 WHERE cloud_account_id = ? AND date BETWEEN ? AND ?
		 GROUP BY service_name, region
		 ORDER BY SUM(cost_amount) DESC`,
		cloudAccountID, monthStart, today,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query resources"})
		return
	}
	defer rows.Close()

	type ResourceGroup struct {
		Type     string `json:"type"`
		Label    string `json:"label"`
		Icon     string `json:"icon"`
		Services []struct {
			Name   string  `json:"name"`
			Region string  `json:"region"`
			Cost   float64 `json:"cost"`
		} `json:"services"`
		TotalCost float64 `json:"total_cost"`
	}

	// Classify service names into resource types
	classify := func(svc string) string {
		s := strings.ToLower(svc)
		switch {
		case strings.Contains(s, "ec2") || strings.Contains(s, "compute engine") ||
			strings.Contains(s, "virtual machine") || strings.Contains(s, "vm ") ||
			strings.Contains(s, "app service"):
			return "compute"
		case strings.Contains(s, "rds") || strings.Contains(s, "aurora") ||
			strings.Contains(s, "sql") || strings.Contains(s, "database") ||
			strings.Contains(s, "cloud sql") || strings.Contains(s, "cosmos"):
			return "database"
		case strings.Contains(s, "s3") || strings.Contains(s, "storage") ||
			strings.Contains(s, "blob") || strings.Contains(s, "gcs") ||
			strings.Contains(s, "glacier") || strings.Contains(s, "backup"):
			return "storage"
		case strings.Contains(s, "lambda") || strings.Contains(s, "function") ||
			strings.Contains(s, "cloud function") || strings.Contains(s, "cloud run"):
			return "serverless"
		case strings.Contains(s, "eks") || strings.Contains(s, "aks") ||
			strings.Contains(s, "gke") || strings.Contains(s, "kubernetes") ||
			strings.Contains(s, "container") || strings.Contains(s, "ecs"):
			return "containers"
		case strings.Contains(s, "cloudfront") || strings.Contains(s, "cdn") ||
			strings.Contains(s, "load balancer") || strings.Contains(s, "network") ||
			strings.Contains(s, "vpc") || strings.Contains(s, "dns") ||
			strings.Contains(s, "traffic manager"):
			return "networking"
		default:
			return "other"
		}
	}

	groupMeta := map[string]struct{ label, icon string }{
		"compute":    {"Compute / VMs", "server"},
		"database":   {"Databases", "database"},
		"storage":    {"Storage", "harddrive"},
		"serverless": {"Serverless / Functions", "zap"},
		"containers": {"Containers / K8s", "box"},
		"networking": {"Networking", "globe"},
		"other":      {"Other Services", "layers"},
	}

	groups := map[string]*ResourceGroup{}
	for rows.Next() {
		var svcName, region string
		var cost float64
		if err := rows.Scan(&svcName, &cost, &region); err != nil {
			continue
		}
		rtype := classify(svcName)
		if _, ok := groups[rtype]; !ok {
			meta := groupMeta[rtype]
			groups[rtype] = &ResourceGroup{
				Type:  rtype,
				Label: meta.label,
				Icon:  meta.icon,
			}
		}
		groups[rtype].Services = append(groups[rtype].Services, struct {
			Name   string  `json:"name"`
			Region string  `json:"region"`
			Cost   float64 `json:"cost"`
		}{Name: svcName, Region: region, Cost: cost})
		groups[rtype].TotalCost += cost
	}

	// Order: compute, database, storage, serverless, containers, networking, other
	order := []string{"compute", "database", "storage", "serverless", "containers", "networking", "other"}
	var result []*ResourceGroup
	for _, k := range order {
		if g, ok := groups[k]; ok {
			result = append(result, g)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"cloud_account_id": cloudAccountID,
		"provider":         provider,
		"resource_groups":  result,
	})
}

// GetResourceTiles handles GET /finops/cloud-accounts/:id/tiles
// Returns live resource count tiles for a single cloud account.
func (h *CloudAccountHandler) GetResourceTiles(c *gin.Context) {
	accountID := c.GetString("account_id")
	cloudAccountID := c.Param("id")

	var provider, accountName, encCreds string
	err := h.DB.QueryRow(
		`SELECT provider, account_name, COALESCE(encrypted_credentials,'')
		 FROM cloud_accounts WHERE id=? AND account_id=? AND deleted_at IS NULL`,
		cloudAccountID, accountID,
	).Scan(&provider, &accountName, &encCreds)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "cloud account not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	type tileJSON struct {
		Icon  string `json:"icon"`
		Label string `json:"label"`
		Color string `json:"color"`
		Count int    `json:"count"`
	}

	var tiles []tileJSON
	if encCreds != "" && h.AESKey != "" {
		if plain, err2 := decrypt(encCreds, h.AESKey); err2 == nil {
			var credsMap map[string]string
			if err3 := json.Unmarshal([]byte(plain), &credsMap); err3 == nil {
				for _, t := range buildTilesFromLiveCounts(provider, credsMap) {
					tiles = append(tiles, tileJSON{Icon: t.Icon, Label: t.Label, Color: t.Color, Count: t.Count})
				}
			}
		}
	}
	if tiles == nil {
		tiles = []tileJSON{}
	}

	c.JSON(http.StatusOK, gin.H{
		"cloud_account_id": cloudAccountID,
		"provider":         provider,
		"account_name":     accountName,
		"tiles":            tiles,
	})
}

// SyncCloudAccount handles POST /finops/cloud-accounts/:id/sync
func (h *CloudAccountHandler) SyncCloudAccount(c *gin.Context) {
	accountID := c.GetString("account_id")
	cloudAccountID := c.Param("id")

	// Verify ownership
	var count int
	err := h.DB.QueryRow(
		`SELECT COUNT(*) FROM cloud_accounts WHERE id = ? AND account_id = ? AND deleted_at IS NULL`,
		cloudAccountID, accountID,
	).Scan(&count)
	if err != nil || count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "cloud account not found"})
		return
	}

	if err := h.Scheduler.SyncOne(cloudAccountID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sync failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "sync completed", "cloud_account_id": cloudAccountID})
}
