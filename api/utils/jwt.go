package utils

import (
	"errors"
	"github.com/golang-jwt/jwt/v4"
	"go-ai-study/api/models"
	"strings"
	"time"
)

// Tokens is immutable after construction; no process-wide signing key exists.
type Tokens struct{ key []byte }

func NewTokens(secret string) (*Tokens, error) {
	lower := strings.ToLower(secret)
	if len(secret) < 32 || strings.TrimSpace(secret) != secret {
		return nil, errors.New("JWT_SECRET must be at least 32 characters without surrounding whitespace")
	}
	for _, word := range []string{"secret", "changeme", "change-me", "placeholder", "example", "password", "your-", "replace"} {
		if strings.Contains(lower, word) {
			return nil, errors.New("JWT_SECRET must not be a placeholder")
		}
	}
	unique := map[rune]bool{}
	for _, r := range secret {
		unique[r] = true
	}
	if len(unique) < 12 {
		return nil, errors.New("JWT_SECRET has insufficient variation")
	}
	return &Tokens{key: []byte(secret)}, nil
}
func (t *Tokens) Generate(user models.User) (string, error) {
	if t == nil || user.ID == 0 || strings.TrimSpace(user.Username) == "" {
		return "", errors.New("invalid token identity")
	}
	now := time.Now()
	claims := &models.Claims{Username: user.Username, UserID: user.ID, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)), IssuedAt: jwt.NewNumericDate(now)}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.key)
}
func (t *Tokens) Validate(value string) (*models.Claims, error) {
	if t == nil {
		return nil, errors.New("token authentication unavailable")
	}
	claims := &models.Claims{}
	token, err := jwt.ParseWithClaims(value, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("invalid signing algorithm")
		}
		return t.key, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid || claims.ExpiresAt == nil || claims.UserID == 0 || strings.TrimSpace(claims.Username) == "" {
		return nil, errors.New("invalid or expired token")
	}
	return claims, nil
}
