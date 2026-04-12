package models

import (
	"github.com/golang-jwt/jwt/v4"
)

type Claims struct {
	Username string `json:"username"`
	UserID   uint   `json:"user_id"`
	jwt.RegisteredClaims
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}