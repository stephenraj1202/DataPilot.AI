package utils

import (
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
)

// AuditEvent holds the data for a single audit log entry.
type AuditEvent struct {
	UserID       string      // may be empty for unauthenticated events
	AccountID    string      // may be empty
	ActionType   string      // e.g. "login_success", "subscription_updated"
	ResourceType string      // e.g. "user", "subscription", "database_connection"
	ResourceID   string      // may be empty
	OldValue     interface{} // serialised to JSON; nil if not applicable
	NewValue     interface{} // serialised to JSON; nil if not applicable
	IPAddress    string
	UserAgent    string
}

// LogAuditEvent inserts a record into the audit_logs table.
// Errors are silently swallowed so that audit failures never disrupt the
// primary request flow.
func LogAuditEvent(db *sql.DB, evt AuditEvent) {
	if db == nil {
		return
	}

	id := uuid.New().String()

	// Nullable string helpers
	nullStr := func(s string) interface{} {
		if s == "" {
			return nil
		}
		return s
	}

	// JSON-encode optional before/after values
	jsonEncode := func(v interface{}) interface{} {
		if v == nil {
			return nil
		}
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		return string(b)
	}

	_, _ = db.Exec(
		`INSERT INTO audit_logs
		 (id, user_id, account_id, action_type, resource_type, resource_id,
		  old_value, new_value, ip_address, user_agent, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())`,
		id,
		nullStr(evt.UserID),
		nullStr(evt.AccountID),
		evt.ActionType,
		evt.ResourceType,
		nullStr(evt.ResourceID),
		jsonEncode(evt.OldValue),
		jsonEncode(evt.NewValue),
		nullStr(evt.IPAddress),
		nullStr(evt.UserAgent),
	)
}
