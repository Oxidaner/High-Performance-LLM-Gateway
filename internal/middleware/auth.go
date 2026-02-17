package middleware

import (
	"context"       //上下文管理
	"crypto/sha256" //哈希算法
	"encoding/hex"  //编码/解码十六进制字符串`
	"net/http"      //HTTP 协议常量和函数
	"strings"       //字符串操作

	"llm-gateway/internal/storage" // Storage layer
	"llm-gateway/pkg/errors"

	"github.com/gin-gonic/gin" // Web framework
)

// APIKeyAuth validates API keys and stores key info in context 用于验证 API 密钥的中间件
func APIKeyAuth(redisClient *storage.RedisClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth if Redis is not available (dev mode)
		// 如果 Redis 客户端为 nil（开发模式），则跳过认证，继续处理请求
		if redisClient == nil {
			c.Next()
			return
		}
		// 检查请求头中是否有 Authorization 头，如果没有则返回 401 错误
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			errors.InvalidRequest("Missing authorization header").JSON(c)
			c.Abort()
			return
		}

		// Extract Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			errors.InvalidRequest("Invalid authorization header format").JSON(c)
			c.Abort()
			return
		}

		apiKey := parts[1]

		// Check API key (with Redis cache)
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
