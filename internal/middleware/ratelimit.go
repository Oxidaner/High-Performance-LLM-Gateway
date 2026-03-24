package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"llm-gateway/internal/config"
	"llm-gateway/pkg/errors"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimiter implements in-memory token bucket limiting.
type RateLimiter struct {
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new in-memory token bucket limiter.
func NewRateLimiter(qps, burst int) *RateLimiter {
	return &RateLimiter{
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		refillRate: float64(qps),
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed.
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

// RedisRateLimiter uses Redis for distributed rate limiting.
type RedisRateLimiter struct {
	client *redis.Client
	key    string
	limit  int
	window time.Duration
}

// NewRedisRateLimiter creates a Redis-based rate limiter.
func NewRedisRateLimiter(client *redis.Client, key string, limit int, window time.Duration) *RedisRateLimiter {
	return &RedisRateLimiter{
		client: client,
		key:    key,
		limit:  limit,
		window: window,
	}
}

// Allow checks if request is allowed under rate limit.
func (r *RedisRateLimiter) Allow() (bool, error) {
	ctx := context.Background()

	count, err := r.client.Incr(ctx, r.key).Result()
	if err != nil {
		return false, err
	}

	if count == 1 {
		r.client.Expire(ctx, r.key, r.window)
	}

	return count <= int64(r.limit), nil
}

// RateLimit applies global and model-level token bucket limiting.
func RateLimit(cfg config.RateLimitConfig) gin.HandlerFunc {
	globalLimiter := NewRateLimiter(cfg.GlobalQPS, cfg.Burst)
	modelLimiters := make(map[string]*RateLimiter, len(cfg.ModelLimits))
	for model, limit := range cfg.ModelLimits {
		if limit <= 0 {
			continue
		}
		modelLimiters[model] = NewRateLimiter(limit, limit)
	}

	return func(c *gin.Context) {
		model := extractModelFromRequest(c)
		if model == "" {
			model = "default"
		}

		if limiter, ok := modelLimiters[model]; ok && !limiter.Allow() {
			err := errors.RateLimitExceeded("Rate limit exceeded for model " + model)
			c.Header("Retry-After", strconv.Itoa(1))
			err.JSON(c)
			c.Abort()
			return
		}

		if !globalLimiter.Allow() {
			err := errors.RateLimitExceeded("Rate limit exceeded")
			c.Header("Retry-After", strconv.Itoa(1))
			err.JSON(c)
			c.Abort()
			return
		}

		c.Next()
	}
}

func extractModelFromRequest(c *gin.Context) string {
	if queryModel := strings.TrimSpace(c.Query("model")); queryModel != "" {
		return queryModel
	}

	if formModel := strings.TrimSpace(c.PostForm("model")); formModel != "" {
		return formModel
	}

	if c.Request == nil || c.Request.Body == nil {
		return ""
	}

	contentType := strings.ToLower(c.GetHeader("Content-Type"))
	if !strings.Contains(contentType, "application/json") {
		return ""
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}

	return strings.TrimSpace(payload.Model)
}
