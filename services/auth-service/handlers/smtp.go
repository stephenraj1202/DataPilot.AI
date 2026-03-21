package handlers

import (
	"database/sql"
	"net/http"

	"github.com/finops-platform/auth-service/middleware"
	"github.com/finops-platform/auth-service/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SMTPHandler handles SMTP configuration endpoints.
type SMTPHandler struct {
	DB     *sql.DB
	AESKey string
}

type smtpRequest struct {
	SMTPHost     string `json:"smtp_host"     binding:"required"`
	SMTPPort     int    `json:"smtp_port"     binding:"required"`
	SMTPUsername string `json:"smtp_username" binding:"required"`
	Password     string `json:"password"      binding:"required"`
	FromEmail    string `json:"from_email"    binding:"required,email"`
	UseTLS       bool   `json:"use_tls"`
}

type smtpResponse struct {
	ID           string `json:"id"`
	AccountID    string `json:"account_id"`
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	FromEmail    string `json:"from_email"`
	UseTLS       bool   `json:"use_tls"`
}

// SaveSMTPConfig handles POST /settings/smtp — creates or updates SMTP config for the account.
func (h *SMTPHandler) SaveSMTPConfig(c *gin.Context) {
	accountID, ok := middleware.GetAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req smtpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Encrypt the SMTP password with AES-256.
	encryptedPassword, err := utils.Encrypt(req.Password, h.AESKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt password"})
		return
	}

	// Upsert: update if a record already exists for this account, otherwise insert.
	var existingID string
	err = h.DB.QueryRow(
		`SELECT id FROM mail_settings WHERE account_id = ?`, accountID,
	).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Insert new record.
		newID := uuid.New().String()
		_, err = h.DB.Exec(
			`INSERT INTO mail_settings
			 (id, account_id, smtp_host, smtp_port, smtp_username, encrypted_password, from_email, use_tls)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			newID, accountID, req.SMTPHost, req.SMTPPort, req.SMTPUsername,
			encryptedPassword, req.FromEmail, req.UseTLS,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save SMTP configuration"})
			return
		}
		existingID = newID
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	} else {
		// Update existing record.
		_, err = h.DB.Exec(
			`UPDATE mail_settings
			 SET smtp_host = ?, smtp_port = ?, smtp_username = ?, encrypted_password = ?,
			     from_email = ?, use_tls = ?, updated_at = NOW()
			 WHERE account_id = ?`,
			req.SMTPHost, req.SMTPPort, req.SMTPUsername, encryptedPassword,
			req.FromEmail, req.UseTLS, accountID,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update SMTP configuration"})
			return
		}
	}

	c.JSON(http.StatusOK, smtpResponse{
		ID:           existingID,
		AccountID:    accountID,
		SMTPHost:     req.SMTPHost,
		SMTPPort:     req.SMTPPort,
		SMTPUsername: req.SMTPUsername,
		FromEmail:    req.FromEmail,
		UseTLS:       req.UseTLS,
	})
}

// GetSMTPConfig handles GET /settings/smtp — returns the current SMTP config (password omitted).
func (h *SMTPHandler) GetSMTPConfig(c *gin.Context) {
	accountID, ok := middleware.GetAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var resp smtpResponse
	err := h.DB.QueryRow(
		`SELECT id, account_id, smtp_host, smtp_port, smtp_username, from_email, use_tls
		 FROM mail_settings WHERE account_id = ?`,
		accountID,
	).Scan(&resp.ID, &resp.AccountID, &resp.SMTPHost, &resp.SMTPPort,
		&resp.SMTPUsername, &resp.FromEmail, &resp.UseTLS)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "no SMTP configuration found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	c.JSON(http.StatusOK, resp)
}
