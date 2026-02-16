package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"llm-gateway/internal/storage"
)

// APIKeyAuth validates API keys
func APIKeyAuth(redisClient *storage.RedisClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth if Redis is not available (dev mode)
		if redisClient == nil {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "Missing authorization header",
					"type":   "invalid_request_error",
					"code":   "missing_authorization",
				},
			})
			c.Abort()
			return
		}

		// Extract Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "Invalid authorization header format",
					"type":   "invalid_request_error",
					"code":   "invalid_authorization",
				},
			})
			c.Abort()
			return
		}

		apiKey := parts[1]

		// Check API key (with Redis cache)
		keyHash := hashAPIKey(apiKey)
		keyInfo, err := checkAPIKey(c.Request.Context(), redisClient, keyHash)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"message": "Internal server error",
					"type":   "server_error",
				},
			})
			c.Abort()
			return
		}

		if keyInfo == nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "Invalid API key",
					"type":   "invalid_api_key",
					"code":   "invalid_api_key",
				},
			})
			c.Abort()
			return
		}

		// Store key info in context
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

	// Try Redis cache first
	cached, err := redisClient.HGetAll(ctx, cacheKey)
	if err == nil && len(cached) > 0 {
		return cached, nil
	}

	// Cache miss - would need to query PostgreSQL
	// For now, return nil (will need postgres client passed in)
	return nil, nil
}

// CORS middleware
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Logger middleware
func Logger() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health"},
	})
}

// Recovery middleware
func Recovery() gin.HandlerFunc {
	return gin.Recovery()
}
