package utils

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecretKey = "test-secret-key-for-unit-tests"

func newTestJWTService() *JWTService {
	return &JWTService{SecretKey: testSecretKey}
}

// --- GenerateAccessToken ---

func TestGenerateAccessToken_ContainsCorrectClaims(t *testing.T) {
	svc := newTestJWTService()
	userID := "user-123"
	accountID := "account-456"
	roles := []string{"admin", "user"}

	tokenStr, err := svc.GenerateAccessToken(userID, accountID, roles)
	require.NoError(t, err)
	require.NotEmpty(t, tokenStr)

	claims, err := svc.ValidateAccessToken(tokenStr)
	require.NoError(t, err)

	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, accountID, claims.AccountID)
	assert.Equal(t, roles, claims.Roles)
	assert.Equal(t, AccessToken, claims.TokenType)
}

func TestGenerateAccessToken_ExpiresIn15Minutes(t *testing.T) {
	svc := newTestJWTService()

	before := time.Now()
	tokenStr, err := svc.GenerateAccessToken("u1", "a1", nil)
	require.NoError(t, err)

	claims, err := svc.ValidateAccessToken(tokenStr)
	require.NoError(t, err)

	expiry := claims.ExpiresAt.Time
	expectedExpiry := before.Add(15 * time.Minute)

	// Allow 5-second tolerance for test execution time
	assert.WithinDuration(t, expectedExpiry, expiry, 5*time.Second)
}

// --- GenerateRefreshToken ---

func TestGenerateRefreshToken_ContainsCorrectClaims(t *testing.T) {
	svc := newTestJWTService()
	userID := "user-789"
	accountID := "account-012"
	roles := []string{"account_owner"}

	tokenStr, err := svc.GenerateRefreshToken(userID, accountID, roles)
	require.NoError(t, err)
	require.NotEmpty(t, tokenStr)

	claims, err := svc.ValidateRefreshToken(tokenStr)
	require.NoError(t, err)

	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, accountID, claims.AccountID)
	assert.Equal(t, roles, claims.Roles)
	assert.Equal(t, RefreshToken, claims.TokenType)
}

func TestGenerateRefreshToken_ExpiresIn7Days(t *testing.T) {
	svc := newTestJWTService()

	before := time.Now()
	tokenStr, err := svc.GenerateRefreshToken("u1", "a1", nil)
	require.NoError(t, err)

	claims, err := svc.ValidateRefreshToken(tokenStr)
	require.NoError(t, err)

	expiry := claims.ExpiresAt.Time
	expectedExpiry := before.Add(7 * 24 * time.Hour)

	assert.WithinDuration(t, expectedExpiry, expiry, 5*time.Second)
}

// --- Token type enforcement ---

func TestValidateAccessToken_RejectsRefreshToken(t *testing.T) {
	svc := newTestJWTService()

	refreshToken, err := svc.GenerateRefreshToken("u1", "a1", nil)
	require.NoError(t, err)

	_, err = svc.ValidateAccessToken(refreshToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected access token")
}

func TestValidateRefreshToken_RejectsAccessToken(t *testing.T) {
	svc := newTestJWTService()

	accessToken, err := svc.GenerateAccessToken("u1", "a1", nil)
	require.NoError(t, err)

	_, err = svc.ValidateRefreshToken(accessToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected refresh token")
}

// --- Expiry validation ---

func TestValidateToken_RejectsExpiredToken(t *testing.T) {
	svc := newTestJWTService()

	// Manually craft an already-expired token
	claims := JWTClaims{
		UserID:    "u1",
		AccountID: "a1",
		Roles:     nil,
		TokenType: AccessToken,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Minute)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Minute)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(testSecretKey))
	require.NoError(t, err)

	_, err = svc.ValidateToken(tokenStr)
	assert.Error(t, err)
}

// --- Invalid token rejection ---

func TestValidateToken_RejectsMalformedToken(t *testing.T) {
	svc := newTestJWTService()

	_, err := svc.ValidateToken("this.is.not.a.valid.jwt")
	assert.Error(t, err)
}

func TestValidateToken_RejectsEmptyToken(t *testing.T) {
	svc := newTestJWTService()

	_, err := svc.ValidateToken("")
	assert.Error(t, err)
}

func TestValidateToken_RejectsWrongSignature(t *testing.T) {
	svc := newTestJWTService()

	// Sign with a different secret
	otherSvc := &JWTService{SecretKey: "completely-different-secret"}
	tokenStr, err := otherSvc.GenerateAccessToken("u1", "a1", nil)
	require.NoError(t, err)

	_, err = svc.ValidateToken(tokenStr)
	assert.Error(t, err)
}

func TestValidateToken_RejectsWrongSigningMethod(t *testing.T) {
	svc := newTestJWTService()

	// Craft a token signed with RS256 (asymmetric) — we use none here to simulate wrong method
	token := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"user_id": "u1",
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	})
	tokenStr, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = svc.ValidateToken(tokenStr)
	assert.Error(t, err)
}

// --- Empty roles ---

func TestGenerateAccessToken_WithEmptyRoles(t *testing.T) {
	svc := newTestJWTService()

	tokenStr, err := svc.GenerateAccessToken("u1", "a1", []string{})
	require.NoError(t, err)

	claims, err := svc.ValidateAccessToken(tokenStr)
	require.NoError(t, err)
	assert.Equal(t, "u1", claims.UserID)
	assert.Equal(t, "a1", claims.AccountID)
}
