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

// APIKeyAuth validates API keys and stores key info in context 用于验证 API 密钥的中间件并在上下文中存储密钥信息
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
			c.Abort() // 终止请求处理流程
			return
		}

		// Extract Bearer token from header 从请求头中提取 Bearer 令牌 (形如 "Bearer <token>")
		parts := strings.SplitN(authHeader, " ", 2) // 按空格分割字符串，最多分割为 2 部分
		if len(parts) != 2 || parts[0] != "Bearer" {
			errors.InvalidRequest("Invalid authorization header format").JSON(c)
			c.Abort() // 终止请求处理流程
			return
		}

		apiKey := parts[1]

		// Check API key (with Redis cache)
		keyHash := hashAPIKey(apiKey) // 计算 API 密钥的哈希值
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
	hash := sha256.Sum256([]byte(apiKey)) // 计算 API 密钥的 SHA-256 哈希值 , sum256 返回一个 32 字节的数组
	return hex.EncodeToString(hash[:])    // 将哈希值转换为十六进制字符串并返回
}

func checkAPIKey(ctx context.Context, redisClient *storage.RedisClient, keyHash string) (map[string]string, error) {
	cacheKey := "api_key:" + keyHash

	// Try Redis cache first
	cached, err := redisClient.HGetAll(ctx, cacheKey) // 从 Redis 缓存中获取 API 密钥的详细信息 ctx 是请求上下文，用于取消操作
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
	return gin.Recovery() //gin 自带的恢复中间件，用于处理panic
}
