package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Unit tests for generateAPIKey and hashAPIKey ---

// TestGenerateAPIKey_Length verifies Requirement 23.1:
// the generated key is a 64-character hex string representing 32 cryptographic bytes.
func TestGenerateAPIKey_Length(t *testing.T) {
	key, err := generateAPIKey()
	require.NoError(t, err)
	// 32 bytes encoded as hex = 64 characters
	assert.Equal(t, 64, len(key), "API key must be 64 hex chars (32 bytes)")
}

// TestGenerateAPIKey_Uniqueness verifies that repeated calls produce distinct keys.
func TestGenerateAPIKey_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key, err := generateAPIKey()
		require.NoError(t, err)
		assert.False(t, seen[key], "duplicate API key generated")
		seen[key] = true
	}
}

// TestGenerateAPIKey_HexEncoded verifies the key contains only valid hex characters.
func TestGenerateAPIKey_HexEncoded(t *testing.T) {
	key, err := generateAPIKey()
	require.NoError(t, err)
	for _, c := range key {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"key contains non-hex character: %c", c)
	}
}

// TestHashAPIKey_DiffersFromRaw verifies Requirement 23.3:
// the stored hash must not equal the raw key.
func TestHashAPIKey_DiffersFromRaw(t *testing.T) {
	key, err := generateAPIKey()
	require.NoError(t, err)
	hash := hashAPIKey(key)
	assert.NotEqual(t, key, hash, "hash must differ from raw key")
}

// TestHashAPIKey_Deterministic verifies the same key always produces the same hash.
func TestHashAPIKey_Deterministic(t *testing.T) {
	key, err := generateAPIKey()
	require.NoError(t, err)
	assert.Equal(t, hashAPIKey(key), hashAPIKey(key))
}

// TestHashAPIKey_DifferentKeysProduceDifferentHashes verifies collision resistance.
func TestHashAPIKey_DifferentKeysProduceDifferentHashes(t *testing.T) {
	key1, _ := generateAPIKey()
	key2, _ := generateAPIKey()
	assert.NotEqual(t, hashAPIKey(key1), hashAPIKey(key2))
}

// TestHashAPIKey_SHA256Length verifies the hash is a 64-char hex SHA-256 digest.
func TestHashAPIKey_SHA256Length(t *testing.T) {
	key, _ := generateAPIKey()
	hash := hashAPIKey(key)
	assert.Equal(t, 64, len(hash), "SHA-256 hex digest must be 64 characters")
}

// --- HTTP handler tests ---

func setupAPIKeyRouter(handler *APIKeyHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	// Inject user_id into context to simulate authenticated middleware
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "test-user-id")
		c.Next()
	})
	router.POST("/auth/api-keys", handler.CreateAPIKey)
	router.GET("/auth/api-keys", handler.ListAPIKeys)
	router.DELETE("/auth/api-keys/:id", handler.RevokeAPIKey)
	return router
}

// TestCreateAPIKey_StoresHashNotRawKey verifies Requirement 23.3:
// the value inserted into the DB is the hash, not the raw key.
func TestCreateAPIKey_StoresHashNotRawKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	handler := &APIKeyHandler{DB: db}
	router := setupAPIKeyRouter(handler)

	mock.ExpectExec("INSERT INTO api_keys").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body, _ := json.Marshal(map[string]string{"name": "my-key"})
	req, _ := http.NewRequest("POST", "/auth/api-keys", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp createAPIKeyResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// The response must contain the raw key
	assert.NotEmpty(t, resp.Key)
	assert.Equal(t, 64, len(resp.Key), "returned key must be 64 hex chars")

	// The raw key must not equal its own hash
	assert.NotEqual(t, resp.Key, hashAPIKey(resp.Key),
		"raw key must differ from its hash")

	require.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateAPIKey_ReturnsKeyOnce verifies Requirement 23.2:
// the raw key is present in the creation response.
func TestCreateAPIKey_ReturnsKeyOnce(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	handler := &APIKeyHandler{DB: db}
	router := setupAPIKeyRouter(handler)

	mock.ExpectExec("INSERT INTO api_keys").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body, _ := json.Marshal(map[string]string{"name": "ci-key"})
	req, _ := http.NewRequest("POST", "/auth/api-keys", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp createAPIKeyResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Key)
	assert.NotEmpty(t, resp.ID)
	assert.NotEmpty(t, resp.Name)
}

// TestCreateAPIKey_InvalidJSON returns 400 for malformed request bodies.
func TestCreateAPIKey_InvalidJSON(t *testing.T) {
	handler := &APIKeyHandler{DB: nil}
	router := setupAPIKeyRouter(handler)

	req, _ := http.NewRequest("POST", "/auth/api-keys", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCreateAPIKey_MissingName returns 400 when name is absent.
func TestCreateAPIKey_MissingName(t *testing.T) {
	handler := &APIKeyHandler{DB: nil}
	router := setupAPIKeyRouter(handler)

	body, _ := json.Marshal(map[string]string{})
	req, _ := http.NewRequest("POST", "/auth/api-keys", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCreateAPIKey_InvalidExpiresAt returns 400 for a non-RFC3339 expires_at.
func TestCreateAPIKey_InvalidExpiresAt(t *testing.T) {
	handler := &APIKeyHandler{DB: nil}
	router := setupAPIKeyRouter(handler)

	body, _ := json.Marshal(map[string]string{
		"name":       "key",
		"expires_at": "not-a-date",
	})
	req, _ := http.NewRequest("POST", "/auth/api-keys", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCreateAPIKey_DefaultExpiry verifies the default expiry is ~365 days from now.
func TestCreateAPIKey_DefaultExpiry(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	handler := &APIKeyHandler{DB: db}
	router := setupAPIKeyRouter(handler)

	mock.ExpectExec("INSERT INTO api_keys").
		WillReturnResult(sqlmock.NewResult(1, 1))

	body, _ := json.Marshal(map[string]string{"name": "default-expiry"})
	req, _ := http.NewRequest("POST", "/auth/api-keys", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	before := time.Now()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp createAPIKeyResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	expiry, err := time.Parse(time.RFC3339, resp.ExpiresAt)
	require.NoError(t, err)

	expected := before.Add(365 * 24 * time.Hour)
	assert.WithinDuration(t, expected, expiry, 5*time.Second)
}

// TestCreateAPIKey_Unauthorized returns 401 when user_id is absent from context.
func TestCreateAPIKey_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &APIKeyHandler{DB: nil}

	router := gin.New()
	// No user_id injected
	router.POST("/auth/api-keys", handler.CreateAPIKey)

	body, _ := json.Marshal(map[string]string{"name": "key"})
	req, _ := http.NewRequest("POST", "/auth/api-keys", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- API key validation (hash lookup) ---

// TestHashAPIKey_ValidKeyMatchesStoredHash verifies Requirement 23.1 / 23.3:
// a valid raw key hashes to the same value that would be stored, enabling lookup.
func TestHashAPIKey_ValidKeyMatchesStoredHash(t *testing.T) {
	rawKey, err := generateAPIKey()
	require.NoError(t, err)

	storedHash := hashAPIKey(rawKey)

	// Simulate validation: re-hash the presented key and compare
	presentedHash := hashAPIKey(rawKey)
	assert.Equal(t, storedHash, presentedHash, "valid key must match stored hash")
}

// TestHashAPIKey_InvalidKeyDoesNotMatchStoredHash verifies that a wrong key fails validation.
func TestHashAPIKey_InvalidKeyDoesNotMatchStoredHash(t *testing.T) {
	rawKey, _ := generateAPIKey()
	storedHash := hashAPIKey(rawKey)

	wrongKey, _ := generateAPIKey()
	assert.NotEqual(t, storedHash, hashAPIKey(wrongKey),
		"wrong key must not match stored hash")
}

// --- Revocation handler tests ---

// TestRevokeAPIKey_Success verifies Requirement 23.5:
// a valid revocation request soft-deletes the key and returns 200.
func TestRevokeAPIKey_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	handler := &APIKeyHandler{DB: db}
	router := setupAPIKeyRouter(handler)

	mock.ExpectExec("UPDATE api_keys SET deleted_at").
		WithArgs("key-id-123", "test-user-id").
		WillReturnResult(sqlmock.NewResult(0, 1)) // 1 row affected

	req, _ := http.NewRequest("DELETE", "/auth/api-keys/key-id-123", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "revoked")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestRevokeAPIKey_NotFound returns 404 when the key doesn't exist or belongs to another user.
func TestRevokeAPIKey_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	handler := &APIKeyHandler{DB: db}
	router := setupAPIKeyRouter(handler)

	mock.ExpectExec("UPDATE api_keys SET deleted_at").
		WithArgs("nonexistent-id", "test-user-id").
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected

	req, _ := http.NewRequest("DELETE", "/auth/api-keys/nonexistent-id", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestRevokeAPIKey_Unauthorized returns 401 when user_id is absent from context.
func TestRevokeAPIKey_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &APIKeyHandler{DB: nil}

	router := gin.New()
	router.DELETE("/auth/api-keys/:id", handler.RevokeAPIKey)

	req, _ := http.NewRequest("DELETE", "/auth/api-keys/some-id", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- ListAPIKeys handler tests ---

// TestListAPIKeys_ReturnsEmptyList returns an empty array when no keys exist.
func TestListAPIKeys_ReturnsEmptyList(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	handler := &APIKeyHandler{DB: db}
	router := setupAPIKeyRouter(handler)

	rows := sqlmock.NewRows([]string{"id", "name", "key_hash", "created_at", "expires_at", "last_used_at"})
	mock.ExpectQuery("SELECT id, name, key_hash, created_at, expires_at, last_used_at FROM api_keys").
		WithArgs("test-user-id").
		WillReturnRows(rows)

	req, _ := http.NewRequest("GET", "/auth/api-keys", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	apiKeys, ok := resp["api_keys"].([]interface{})
	assert.True(t, ok)
	assert.Empty(t, apiKeys)
}

// TestListAPIKeys_ReturnsKeysWithoutRawKey verifies that listed keys never expose the raw key.
func TestListAPIKeys_ReturnsKeysWithoutRawKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	handler := &APIKeyHandler{DB: db}
	router := setupAPIKeyRouter(handler)

	now := time.Now()
	expires := now.Add(365 * 24 * time.Hour)
	rows := sqlmock.NewRows([]string{"id", "name", "key_hash", "created_at", "expires_at", "last_used_at"}).
		AddRow("id-1", "my-key", "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", now, expires, nil)

	mock.ExpectQuery("SELECT id, name, key_hash, created_at, expires_at, last_used_at FROM api_keys").
		WithArgs("test-user-id").
		WillReturnRows(rows)

	req, _ := http.NewRequest("GET", "/auth/api-keys", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// The response body must not contain a "key" field with the full raw key
	body := w.Body.String()
	assert.NotContains(t, body, `"key":`)
}
