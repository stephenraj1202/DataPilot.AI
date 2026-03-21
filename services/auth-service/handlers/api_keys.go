package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// APIKeyHandler handles API key management endpoints
type APIKeyHandler struct {
	DB *sql.DB
}

type createAPIKeyRequest struct {
	Name      string `json:"name" binding:"required"`
	ExpiresAt string `json:"expires_at"` // optional, RFC3339 format
}

type apiKeyResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	KeyPrefix  string  `json:"key_prefix"` // first 8 chars for identification
	CreatedAt  string  `json:"created_at"`
	ExpiresAt  string  `json:"expires_at"`
	LastUsedAt *string `json:"last_used_at"`
}

type createAPIKeyResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Key       string `json:"key"` // only returned on creation
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
}

// CreateAPIKey generates a new 32-character cryptographically secure API key,
// stores its SHA-256 hash, and returns the plaintext key once.
func (h *APIKeyHandler) CreateAPIKey(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Generate 32-byte (256-bit) random key, encode as 64-char hex string
	rawKey, err := generateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate API key"})
		return
	}

	// Hash the key for storage
	keyHash := hashAPIKey(rawKey)

	// Determine expiry
	var expiresAt time.Time
	if req.ExpiresAt != "" {
		expiresAt, err = time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid expires_at format, use RFC3339"})
			return
		}
	} else {
		// Default: 1 year
		expiresAt = time.Now().Add(365 * 24 * time.Hour)
	}

	id := uuid.New().String()
	now := time.Now()

	_, err = h.DB.Exec(
		`INSERT INTO api_keys (id, user_id, key_hash, name, expires_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, userID.(string), keyHash, req.Name, expiresAt, now, now,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store API key"})
		return
	}

	c.JSON(http.StatusCreated, createAPIKeyResponse{
		ID:        id,
		Name:      req.Name,
		Key:       rawKey,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
		CreatedAt: now.UTC().Format(time.RFC3339),
	})
}

// ListAPIKeys returns all active API keys for the authenticated user
func (h *APIKeyHandler) ListAPIKeys(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	rows, err := h.DB.Query(
		`SELECT id, name, key_hash, created_at, expires_at, last_used_at
		 FROM api_keys
		 WHERE user_id = ? AND deleted_at IS NULL
		 ORDER BY created_at DESC`,
		userID.(string),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var keys []apiKeyResponse
	for rows.Next() {
		var (
			id, name, keyHash    string
			createdAt, expiresAt time.Time
			lastUsedAt           sql.NullTime
		)
		if err := rows.Scan(&id, &name, &keyHash, &createdAt, &expiresAt, &lastUsedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read API keys"})
			return
		}

		var lastUsedStr *string
		if lastUsedAt.Valid {
			s := lastUsedAt.Time.UTC().Format(time.RFC3339)
			lastUsedStr = &s
		}

		// Show only first 8 chars of hash as a prefix identifier
		prefix := keyHash
		if len(prefix) > 8 {
			prefix = prefix[:8]
		}

		keys = append(keys, apiKeyResponse{
			ID:         id,
			Name:       name,
			KeyPrefix:  prefix,
			CreatedAt:  createdAt.UTC().Format(time.RFC3339),
			ExpiresAt:  expiresAt.UTC().Format(time.RFC3339),
			LastUsedAt: lastUsedStr,
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if keys == nil {
		keys = []apiKeyResponse{}
	}

	c.JSON(http.StatusOK, gin.H{"api_keys": keys})
}

// RevokeAPIKey soft-deletes an API key by setting deleted_at
func (h *APIKeyHandler) RevokeAPIKey(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	keyID := c.Param("id")
	if keyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API key ID is required"})
		return
	}

	result, err := h.DB.Exec(
		`UPDATE api_keys SET deleted_at = NOW(), updated_at = NOW()
		 WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		keyID, userID.(string),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key revoked successfully"})
}

// generateAPIKey creates a 32-byte cryptographically secure random key
// and returns it as a 64-character hex string.
func generateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// hashAPIKey returns the SHA-256 hex digest of the given key.
func hashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// UpdateAPIKeyLastUsed updates the last_used_at timestamp for an API key identified by its hash.
// This is called by the authentication middleware when an API key is used.
func UpdateAPIKeyLastUsed(db *sql.DB, keyHash string) {
	if db == nil {
		return
	}
	_, _ = db.Exec(
		`UPDATE api_keys SET last_used_at = NOW(), updated_at = NOW()
		 WHERE key_hash = ? AND deleted_at IS NULL`,
		keyHash,
	)
}
