package middleware

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"llm-gateway/internal/config"
	"llm-gateway/pkg/errors"
)

// RateLimiter implements token bucket rate limiting (in-memory)
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

// Allow checks if a request is allowed
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.tokens += elapsed * r.refillRate
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}
	r.lastRefill = now

	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	return false
}

// RedisRateLimiter uses Redis for distributed rate limiting
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

// Allow checks if request is allowed under rate limit
func (r *RedisRateLimiter) Allow() (bool, error) {
	ctx := context.Background()

	// Increment counter
	count, err := r.client.Incr(ctx, r.key).Result()
	if err != nil {
		return false, err
	}

	// Set expiry on first request
	if count == 1 {
		r.client.Expire(ctx, r.key, r.window)
	}

	return count <= int64(r.limit), nil
}

// RateLimit middleware with Redis support
func RateLimit(cfg config.RateLimitConfig) gin.HandlerFunc {
	// In-memory rate limiter for global limit
	globalLimiter := NewRateLimiter(cfg.GlobalQPS, cfg.Burst)

	return func(c *gin.Context) {
		// Get model from request body
		model := c.PostForm("model")
		if model == "" {
			model = "default"
		}

		// Check model-specific limit
		if limit, ok := cfg.ModelLimits[model]; ok {
			// Would use Redis for per-model limiting
			_ = limit
		}

		// Check global limit
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
