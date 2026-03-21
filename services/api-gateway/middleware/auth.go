package middleware

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// jwtClaims mirrors the claims produced by the Auth Service.
type jwtClaims struct {
	UserID    string   `json:"user_id"`
	AccountID string   `json:"account_id"`
	Roles     []string `json:"roles"`
	TokenType string   `json:"token_type"`
	jwt.RegisteredClaims
}

// AuthMiddleware validates JWT access tokens or API keys on every request.
type AuthMiddleware struct {
	JWTSecret string
	DB        *sql.DB
}

// Authenticate is a Gin middleware that enforces authentication.
// It accepts either:
//   - Authorization: Bearer <jwt>
//   - X-API-Key: <raw-api-key>
func (m *AuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Try API key first
		if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
			if m.validateAPIKey(c, apiKey) {
				c.Next()
				return
			}
			// validateAPIKey already wrote the 401 response
			return
		}

		// 2. Try Bearer JWT
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format, expected 'Bearer <token>'"})
			c.Abort()
			return
		}

		claims, err := parseJWT(parts[1], m.JWTSecret)
		if err != nil || claims.TokenType != "access" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("account_id", claims.AccountID)
		c.Set("roles", claims.Roles)
		c.Next()
	}
}

// validateAPIKey looks up the hashed key in the database and populates context.
// Returns true on success; on failure it writes the 401 and aborts.
func (m *AuthMiddleware) validateAPIKey(c *gin.Context, rawKey string) bool {
	keyHash := hashKey(rawKey)

	var userID, accountID string
	var expiresAt time.Time

	err := m.DB.QueryRow(
		`SELECT ak.user_id, u.account_id, ak.expires_at
		 FROM api_keys ak
		 JOIN users u ON u.id = ak.user_id
		 WHERE ak.key_hash = ?
		   AND ak.deleted_at IS NULL
		   AND u.deleted_at IS NULL`,
		keyHash,
	).Scan(&userID, &accountID, &expiresAt)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
		c.Abort()
		return false
	}

	if time.Now().After(expiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "API key has expired"})
		c.Abort()
		return false
	}

	// Update last_used_at asynchronously — don't block the request
	go func() {
		_, _ = m.DB.Exec(
			`UPDATE api_keys SET last_used_at = NOW(), updated_at = NOW() WHERE key_hash = ?`,
			keyHash,
		)
	}()

	// Fetch roles for the user
	roles := fetchRoles(m.DB, userID)

	c.Set("user_id", userID)
	c.Set("account_id", accountID)
	c.Set("roles", roles)
	return true
}

// parseJWT validates the token string and returns the claims.
func parseJWT(tokenStr, secret string) (*jwtClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

// hashKey returns the SHA-256 hex digest of the raw API key.
func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// fetchRoles returns the role names for a user.
func fetchRoles(db *sql.DB, userID string) []string {
	rows, err := db.Query(
		`SELECT r.name FROM roles r
		 JOIN user_roles ur ON ur.role_id = r.id
		 WHERE ur.user_id = ?`, userID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var roles []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			roles = append(roles, name)
		}
	}
	return roles
}
