package middleware

import (
	"net/http"
	"strings"

	"million-rps/internal/config"
	"million-rps/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		auth := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if auth == "" || !strings.HasPrefix(auth, prefix) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			logger.Debug(ctx, "Missing or invalid Authorization header")
			c.Abort()
			return
		}
		tokenStr := strings.TrimSpace(auth[len(prefix):])
		secret := config.GetJWTSecret(ctx)
		if secret == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server misconfiguration"})
			c.Abort()
			return
		}
		claims, err := jwt.ParseWithClaims(tokenStr, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			logger.Debug(ctx, "JWT parse failed", "error", err)
			c.Abort()
			return
		}
		if !claims.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}
		c.Set("user", claims.Claims.(*jwt.RegisteredClaims).Subject)
		c.Next()
	}
}
