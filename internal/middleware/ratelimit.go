package middleware

import (
	"context"
	"strconv"
	"sync"
	"time"

	"llm-gateway/internal/config"
	"llm-gateway/pkg/errors"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimiter implements token bucket rate limiting (in-memory) 令牌桶速率限制器 (内存实现)
type RateLimiter struct {
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(qps, burst int) *RateLimiter {
	return &RateLimiter{
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		refillRate: float64(qps),
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed 如果请求被允许，返回true，否则返回false
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds() // 自上次填充以来的时间间隔
	r.tokens += elapsed * r.refillRate         // 令牌桶中的令牌数 = 上次填充以来的时间间隔 * 填充速率
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens // 令牌桶中的令牌数不能超过最大令牌数
	}
	r.lastRefill = now // 更新上次填充时间

	if r.tokens >= 1 { // 如果令牌桶中的令牌数大于等于1
		r.tokens-- // 消耗1个令牌
		return true
	}
	return false
}

// RedisRateLimiter uses Redis for distributed rate limiting 基于Redis的速率限制器
type RedisRateLimiter struct {
	client *redis.Client
	key    string
	limit  int
	window time.Duration
}

// NewRedisRateLimiter creates a Redis-based rate limiter
func NewRedisRateLimiter(client *redis.Client, key string, limit int, window time.Duration) *RedisRateLimiter {
	return &RedisRateLimiter{
		client: client,
		key:    key,
		limit:  limit,
		window: window,
	}
}

// Allow checks if request is allowed under rate limit 检查请求是否在速率限制内
func (r *RedisRateLimiter) Allow() (bool, error) {
	ctx := context.Background()

	// Increment counter
	count, err := r.client.Incr(ctx, r.key).Result()
	if err != nil {
		return false, err
	}

	// Set expiry on first request 第一次请求时设置过期时间
	if count == 1 {
		r.client.Expire(ctx, r.key, r.window)
	}

	return count <= int64(r.limit), nil
}

// RateLimit middleware with Redis support
func RateLimit(cfg config.RateLimitConfig) gin.HandlerFunc {
	// In-memory rate limiter for global limit
	globalLimiter := NewRateLimiter(cfg.GlobalQPS, cfg.Burst) // 全局速率限制器

	return func(c *gin.Context) {
		// Get model from request body
		model := c.PostForm("model")
		if model == "" {
			model = "default"
		}

		// Check model-specific limit
		if limit, ok := cfg.ModelLimits[model]; ok { // 检查模型是否有特定的QPS限制
			// Would use Redis for per-model limiting
			_ = limit
		}

		// Check global limit
		// 如果全局速率限制器不允许请求，则返回错误
		if !globalLimiter.Allow() {
			err := errors.RateLimitExceeded("Rate limit exceeded")
			c.Header("Retry-After", strconv.Itoa(int(cfg.GlobalQPS)))
			err.JSON(c)
			c.Abort()
			return
		}

		c.Next()
	}
}
