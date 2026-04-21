package token

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/uncle3dev/velotrax-auth-go/internal/config"
)

type claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func GenerateAccess(userID string, cfg *config.Config) (string, error) {
	return generate(userID, cfg.JWTExpiry, cfg.JWTSecret)
}

func GenerateRefresh(userID string, cfg *config.Config) (string, error) {
	return generate(userID, cfg.JWTRefreshExpiry, cfg.JWTSecret)
}

func ValidateRefresh(tokenStr string, cfg *config.Config) (string, error) {
	return validate(tokenStr, cfg.JWTSecret)
}

func generate(userID string, expiry time.Duration, secret string) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	return t.SignedString([]byte(secret))
}

func validate(tokenStr, secret string) (string, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !t.Valid {
		return "", fmt.Errorf("invalid token")
	}
	c, ok := t.Claims.(*claims)
	if !ok {
		return "", fmt.Errorf("invalid claims")
	}
	return c.UserID, nil
}
