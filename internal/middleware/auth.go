package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"llm-gateway/internal/storage"
	"llm-gateway/pkg/errors"

	"github.com/gin-gonic/gin"
)

// APIKeyAuth validates API keys and stores key metadata in request context.
func APIKeyAuth(redisClient *storage.RedisClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth if Redis is unavailable (debug mode behavior).
		if redisClient == nil {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			errors.MissingAPIKey("Missing authorization header").JSON(c)
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			errors.InvalidAPIKey("Invalid authorization header format").JSON(c)
			c.Abort()
			return
		}

		apiKey := parts[1]
		keyHash := hashAPIKey(apiKey)
		keyInfo, err := checkAPIKey(c.Request.Context(), redisClient, keyHash)
		if err != nil {
			errors.InternalError("Internal server error").JSON(c)
			c.Abort()
			return
		}

		if keyInfo == nil {
			errors.InvalidAPIKey("Invalid API key").JSON(c)
			c.Abort()
			return
		}

		c.Set("api_key_id", keyInfo["id"])
		c.Set("api_key_name", keyInfo["name"])
		c.Set("api_key_rate_limit", keyInfo["rate_limit"])

		c.Next()
	}
}

func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

func checkAPIKey(ctx context.Context, redisClient *storage.RedisClient, keyHash string) (map[string]string, error) {
	cacheKey := "api_key:" + keyHash

	cached, err := redisClient.HGetAll(ctx, cacheKey)
	if err == nil && len(cached) > 0 {
		return cached, nil
	}

	// Cache miss: PostgreSQL lookup is not wired yet.
	return nil, nil
}

// CORS middleware.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Logger middleware.
func Logger() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health"},
	})
}

// Recovery middleware.
func Recovery() gin.HandlerFunc {
	return gin.Recovery()
}
