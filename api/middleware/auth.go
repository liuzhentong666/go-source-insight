package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go-ai-study/api/utils"
)

func AuthMiddleware(tokens ...*utils.Tokens) gin.HandlerFunc {
	var signer *utils.Tokens
	if len(tokens) > 0 {
		signer = tokens[0]
	}
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, bearerPrefix)
		claims, err := signer.Validate(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// 将用户信息存储在上下文中
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)

		c.Next()
	}
}
