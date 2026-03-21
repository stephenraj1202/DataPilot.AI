package handlers

import (
	"database/sql"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/google/uuid"
)

// logAuditEvent inserts a record into audit_logs; errors are silently ignored.
func logAuditEvent(db *sql.DB, userID, accountID, actionType, resourceType, resourceID, ipAddress, userAgent string) {
	if db == nil {
		return
	}
	id := uuid.New().String()
	var uID, aID, rID interface{}
	if userID != "" {
		uID = userID
	}
	if accountID != "" {
		aID = accountID
	}
	if resourceID != "" {
		rID = resourceID
	}
	_, _ = db.Exec(
		`INSERT INTO audit_logs (id, user_id, account_id, action_type, resource_type, resource_id, ip_address, user_agent, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW())`,
		id, uID, aID, actionType, resourceType, rID, ipAddress, userAgent,
	)
}

// EmailConfig holds SMTP configuration for sending emails.
type EmailConfig struct {
	SMTPHost        string
	SMTPPort        string
	FromEmail       string
	SuperAdminEmail string
}

// sendEmail sends a plain-text email via SMTP.
func sendEmail(cfg EmailConfig, to []string, subject, body string) error {
	toHeader := strings.Join(to, ", ")
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", cfg.FromEmail))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", toHeader))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	addr := fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort)
	return smtp.SendMail(addr, nil, cfg.FromEmail, to, []byte(msg.String()))
}
