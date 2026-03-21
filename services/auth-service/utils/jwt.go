package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenType represents the type of JWT token
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// JWTClaims represents the custom claims for JWT tokens
type JWTClaims struct {
	UserID    string    `json:"user_id"`
	AccountID string    `json:"account_id"`
	Roles     []string  `json:"roles"`
	TokenType TokenType `json:"token_type"`
	Name      string    `json:"name,omitempty"`
	Avatar    string    `json:"avatar,omitempty"`
	Email     string    `json:"email,omitempty"`
	jwt.RegisteredClaims
}

// JWTService handles JWT token generation and validation
type JWTService struct {
	SecretKey string
}

// UserProfile carries optional display fields embedded in the token.
type UserProfile struct {
	Name   string
	Avatar string
	Email  string
}

// GenerateAccessToken creates a new access token with 15-minute expiry.
func (j *JWTService) GenerateAccessToken(userID, accountID string, roles []string) (string, error) {
	return j.generateToken(userID, accountID, roles, UserProfile{}, AccessToken, 15*time.Minute)
}

// GenerateAccessTokenWithProfile creates an access token that includes name/avatar/email.
func (j *JWTService) GenerateAccessTokenWithProfile(userID, accountID string, roles []string, profile UserProfile) (string, error) {
	return j.generateToken(userID, accountID, roles, profile, AccessToken, 15*time.Minute)
}

// GenerateRefreshToken creates a new refresh token with 7-day expiry.
func (j *JWTService) GenerateRefreshToken(userID, accountID string, roles []string) (string, error) {
	return j.generateToken(userID, accountID, roles, UserProfile{}, RefreshToken, 7*24*time.Hour)
}

// GenerateRefreshTokenWithProfile creates a refresh token that includes name/avatar/email.
func (j *JWTService) GenerateRefreshTokenWithProfile(userID, accountID string, roles []string, profile UserProfile) (string, error) {
	return j.generateToken(userID, accountID, roles, profile, RefreshToken, 7*24*time.Hour)
}

// generateToken creates a JWT token with the specified parameters.
func (j *JWTService) generateToken(userID, accountID string, roles []string, profile UserProfile, tokenType TokenType, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		UserID:    userID,
		AccountID: accountID,
		Roles:     roles,
		TokenType: tokenType,
		Name:      profile.Name,
		Avatar:    profile.Avatar,
		Email:     profile.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.SecretKey))
}

// ValidateToken validates a JWT token and returns the claims
func (j *JWTService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(j.SecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// ValidateAccessToken validates an access token specifically
func (j *JWTService) ValidateAccessToken(tokenString string) (*JWTClaims, error) {
	claims, err := j.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != AccessToken {
		return nil, errors.New("invalid token type: expected access token")
	}

	return claims, nil
}

// ValidateRefreshToken validates a refresh token specifically
func (j *JWTService) ValidateRefreshToken(tokenString string) (*JWTClaims, error) {
	claims, err := j.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != RefreshToken {
		return nil, errors.New("invalid token type: expected refresh token")
	}

	return claims, nil
}
