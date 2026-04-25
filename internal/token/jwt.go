package token

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/uncle3dev/velotrax-auth-go/internal/config"
)

type claims struct {
	Type  string   `json:"type"`
	Roles []string `json:"roles,omitempty"`
	jwt.RegisteredClaims
}

func GenerateAccess(userID string, roles []string, cfg *config.Config) (string, error) {
	return generate(userID, roles, "access", cfg.JWTExpiry, cfg.JWTSecret)
}

func GenerateRefresh(userID string, roles []string, cfg *config.Config) (string, error) {
	return generate(userID, roles, "refresh", cfg.JWTRefreshExpiry, cfg.JWTSecret)
}

func ValidateAccess(tokenStr string, cfg *config.Config) (string, []string, error) {
	return validate(tokenStr, cfg.JWTSecret, "access")
}

func ValidateRefresh(tokenStr string, cfg *config.Config) (string, []string, error) {
	return validate(tokenStr, cfg.JWTSecret, "refresh")
}

func generate(userID string, roles []string, tokenType string, expiry time.Duration, secret string) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Type:  tokenType,
		Roles: roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	return t.SignedString([]byte(secret))
}

func validate(tokenStr, secret, expectedType string) (string, []string, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !t.Valid {
		return "", nil, fmt.Errorf("invalid token")
	}
	c, ok := t.Claims.(*claims)
	if !ok {
		return "", nil, fmt.Errorf("invalid claims")
	}
	if c.Type != expectedType {
		return "", nil, fmt.Errorf("invalid token type")
	}
	if c.Subject == "" {
		return "", nil, fmt.Errorf("missing subject")
	}
	return c.Subject, c.Roles, nil
}
