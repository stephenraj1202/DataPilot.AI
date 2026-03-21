package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// AuditHandler serves audit log queries for Super_Admin users.
type AuditHandler struct {
	DB *sql.DB
}

// AuditLogEntry is the JSON representation of a single audit_logs row.
type AuditLogEntry struct {
	ID           string  `json:"id"`
	UserID       *string `json:"user_id"`
	AccountID    *string `json:"account_id"`
	ActionType   string  `json:"action_type"`
	ResourceType string  `json:"resource_type"`
	ResourceID   *string `json:"resource_id"`
	OldValue     *string `json:"old_value"`
	NewValue     *string `json:"new_value"`
	IPAddress    *string `json:"ip_address"`
	UserAgent    *string `json:"user_agent"`
	CreatedAt    string  `json:"created_at"`
}

// ListAuditLogs handles GET /admin/audit-logs
//
// Query parameters (all optional):
//
//	account_id  – filter by account
//	user_id     – filter by user
//	action_type – filter by action type (exact match)
//	start_date  – ISO-8601 date (YYYY-MM-DD), inclusive lower bound
//	end_date    – ISO-8601 date (YYYY-MM-DD), inclusive upper bound
//	limit       – max rows to return (default 100, max 1000)
//	offset      – pagination offset (default 0)
func (h *AuditHandler) ListAuditLogs(c *gin.Context) {
	// --- parse query parameters ---
	accountID := c.Query("account_id")
	userID := c.Query("user_id")
	actionType := c.Query("action_type")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	limit := 100
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			if v > 1000 {
				v = 1000
			}
			limit = v
		}
	}

	offset := 0
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	// --- build dynamic WHERE clause ---
	var conditions []string
	var args []interface{}

	if accountID != "" {
		conditions = append(conditions, "account_id = ?")
		args = append(args, accountID)
	}
	if userID != "" {
		conditions = append(conditions, "user_id = ?")
		args = append(args, userID)
	}
	if actionType != "" {
		conditions = append(conditions, "action_type = ?")
		args = append(args, actionType)
	}
	if startDate != "" {
		if _, err := time.Parse("2006-01-02", startDate); err == nil {
			conditions = append(conditions, "created_at >= ?")
			args = append(args, startDate+" 00:00:00")
		}
	}
	if endDate != "" {
		if _, err := time.Parse("2006-01-02", endDate); err == nil {
			conditions = append(conditions, "created_at <= ?")
			args = append(args, endDate+" 23:59:59")
		}
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// --- count total matching rows ---
	countQuery := "SELECT COUNT(*) FROM audit_logs " + where
	var total int
	if err := h.DB.QueryRow(countQuery, args...).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count audit logs"})
		return
	}

	// --- fetch page ---
	dataQuery := `SELECT id, user_id, account_id, action_type, resource_type,
	                     resource_id, old_value, new_value, ip_address, user_agent,
	                     created_at
	              FROM audit_logs ` + where +
		` ORDER BY created_at DESC LIMIT ? OFFSET ?`

	pageArgs := append(args, limit, offset)
	rows, err := h.DB.Query(dataQuery, pageArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query audit logs"})
		return
	}
	defer rows.Close()

	entries := make([]AuditLogEntry, 0)
	for rows.Next() {
		var e AuditLogEntry
		var createdAt time.Time
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.AccountID, &e.ActionType, &e.ResourceType,
			&e.ResourceID, &e.OldValue, &e.NewValue, &e.IPAddress, &e.UserAgent,
			&createdAt,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan audit log row"})
			return
		}
		e.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error iterating audit logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   entries,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
