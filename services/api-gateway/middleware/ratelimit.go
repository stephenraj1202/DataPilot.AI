package middleware

import (
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// planLimits maps subscription plan names to requests-per-minute.
var planLimits = map[string]int{
	"free":       100,
	"base":       500,
	"pro":        2000,
	"enterprise": 10000,
}

// defaultLimit is used when the plan cannot be determined.
const defaultLimit = 100

// windowEntry tracks the request count and window expiry for one user.
type windowEntry struct {
	count     int64
	expiresAt time.Time
}

// RateLimitMiddleware enforces per-user, per-minute request limits using an in-memory counter.
type RateLimitMiddleware struct {
	DB      *sql.DB
	mu      sync.Mutex
	windows map[string]*windowEntry
}

// Limit returns a Gin middleware that enforces subscription-tier rate limits.
func (m *RateLimitMiddleware) Limit() gin.HandlerFunc {
	m.mu.Lock()
	if m.windows == nil {
		m.windows = make(map[string]*windowEntry)
	}
	m.mu.Unlock()

	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		uid := userID.(string)
		limit := m.getPlanLimit(uid)

		allowed, retryAfter := m.checkAndIncrement(uid, limit)
		if !allowed {
			c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": retryAfter,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// getPlanLimit queries the user's active subscription plan and returns the rpm limit.
func (m *RateLimitMiddleware) getPlanLimit(userID string) int {
	var planName string
	err := m.DB.QueryRow(
		`SELECT sp.name
		 FROM stripe_subscriptions ss
		 JOIN subscription_plans sp ON sp.id = ss.plan_id
		 JOIN stripe_customers sc ON sc.account_id = ss.account_id
		 JOIN users u ON u.account_id = sc.account_id
		 WHERE u.id = ?
		   AND ss.status IN ('active', 'trialing')
		   AND ss.deleted_at IS NULL
		 ORDER BY ss.created_at DESC
		 LIMIT 1`,
		userID,
	).Scan(&planName)
	if err != nil {
		return defaultLimit
	}
	if limit, ok := planLimits[planName]; ok {
		return limit
	}
	return defaultLimit
}

// checkAndIncrement uses an in-memory sliding-window counter (1-minute window).
// Returns (allowed bool, retryAfterSeconds int).
func (m *RateLimitMiddleware) checkAndIncrement(userID string, limit int) (bool, int) {
	now := time.Now()
	windowStart := now.Truncate(time.Minute)
	windowExpiry := windowStart.Add(time.Minute)
	ttl := int(time.Until(windowExpiry).Seconds()) + 1

	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.windows[userID]
	if !ok || now.After(entry.expiresAt) {
		// New window
		m.windows[userID] = &windowEntry{count: 1, expiresAt: windowExpiry}
		return true, 0
	}

	entry.count++
	if entry.count > int64(limit) {
		return false, ttl
	}
	return true, 0
}
