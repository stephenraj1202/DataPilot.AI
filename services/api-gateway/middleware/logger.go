package middleware

import (
	"database/sql"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestLogger logs every request to the audit_logs table.
type RequestLogger struct {
	DB *sql.DB
}

// Log returns a Gin middleware that records request details after the response is written.
func (l *RequestLogger) Log() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start)

		// Collect context values set by auth middleware (may be empty for public routes)
		userID, _ := c.Get("user_id")
		accountID, _ := c.Get("account_id")

		userIDStr, _ := userID.(string)
		accountIDStr, _ := accountID.(string)

		go l.writeLog(
			userIDStr,
			accountIDStr,
			c.Request.Method,
			c.FullPath(),
			c.ClientIP(),
			c.Request.UserAgent(),
			c.Writer.Status(),
			duration.Milliseconds(),
		)
	}
}

func (l *RequestLogger) writeLog(
	userID, accountID, method, endpoint, ipAddress, userAgent string,
	statusCode int,
	responseTimeMs int64,
) {
	if l.DB == nil {
		return
	}
	_, _ = l.DB.Exec(
		`INSERT INTO audit_logs
		 (id, user_id, account_id, action_type, resource_type, resource_id,
		  old_value, new_value, ip_address, user_agent, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, NULL, NULL, ?, ?, NOW())`,
		uuid.New().String(),
		nullableStr(userID),
		nullableStr(accountID),
		method,
		endpoint,
		statusCode,
		ipAddress,
		userAgent,
	)
}

// nullableStr converts an empty string to nil for nullable DB columns.
func nullableStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
