package utils

import (
	"github.com/golang-jwt/jwt/v4"
	"go-ai-study/api/models"
	"testing"
	"time"
)

const testSecret = "N7q4KP2rV8mZ1uX5dT9sB3cW6fH0jLQe"

func TestJWTConfiguredIdentityAndStrictAlgorithm(t *testing.T) {
	signer, err := NewTokens(testSecret)
	if err != nil {
		t.Fatal(err)
	}
	value, err := signer.Generate(models.User{ID: 7, Username: "example-user"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := signer.Validate(value)
	if err != nil || claims.UserID != 7 || claims.Username != "example-user" {
		t.Fatalf("roundtrip=%+v %v", claims, err)
	}
	other, _ := NewTokens("B0m3TX9rN6cV1sL5qW7dK2uH8jP4fZYe")
	if _, err = other.Validate(value); err == nil {
		t.Fatal("wrong key accepted")
	}
	for _, tc := range []struct {
		name   string
		method jwt.SigningMethod
		claims models.Claims
		key    any
	}{
		{"HS384", jwt.SigningMethodHS384, *claims, []byte(testSecret)},
		{"none", jwt.SigningMethodNone, *claims, jwt.UnsafeAllowNoneSignatureType},
		{"expired", jwt.SigningMethodHS256, models.Claims{UserID: 7, Username: "example-user", RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute))}}, []byte(testSecret)},
		{"missing_expiry", jwt.SigningMethodHS256, models.Claims{UserID: 7, Username: "example-user"}, []byte(testSecret)},
		{"missing_id", jwt.SigningMethodHS256, models.Claims{Username: "example-user", RegisteredClaims: claims.RegisteredClaims}, []byte(testSecret)},
		{"missing_name", jwt.SigningMethodHS256, models.Claims{UserID: 7, RegisteredClaims: claims.RegisteredClaims}, []byte(testSecret)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, err := jwt.NewWithClaims(tc.method, &tc.claims).SignedString(tc.key)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = signer.Validate(value); err == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
}
func TestRejectWeakJWTSecrets(t *testing.T) {
	for _, secret := range []string{"", "short", "your-secret-key-here", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "change-me-with-a-long-secret-value", testSecret + " "} {
		if _, err := NewTokens(secret); err == nil {
			t.Fatalf("weak secret accepted %q", secret)
		}
	}
}
