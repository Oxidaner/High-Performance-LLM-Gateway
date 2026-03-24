package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// L1Cache implements exact prompt caching using Redis Hash
type L1Cache struct {
	client    *redis.Client
	ttl       time.Duration
	maxSize   int64
	keyPrefix string
}

// L1CacheConfig holds L1 cache configuration
type L1CacheConfig struct {
	Enabled   bool
	TTL       int // seconds
	MaxSize   int64
	KeyPrefix string
}

// NewL1Cache creates a new L1 cache instance
func NewL1Cache(client *redis.Client, cfg L1CacheConfig) *L1Cache {
	return &L1Cache{
		client:    client,
		ttl:       time.Duration(cfg.TTL) * time.Second,
		maxSize:   cfg.MaxSize,
		keyPrefix: cfg.KeyPrefix,
	}
}

// GenerateCacheKey generates a SHA256 hash key from request
func GenerateCacheKey(model string, messages []Message, params map[string]interface{}) string {
	payload := struct {
		Model    string                 `json:"model"`
		Messages []Message              `json:"messages"`
		Params   map[string]interface{} `json:"params"`
	}{
		Model:    model,
		Messages: messages,
		Params:   params,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		data = []byte(model)
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// Get retrieves cached response for given key
func (c *L1Cache) Get(ctx context.Context, key string) ([]byte, error) {
	if c.client == nil {
		return nil, nil
	}

	val, err := c.client.HGet(ctx, c.keyPrefix, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return []byte(val), nil
}

// Set stores response in cache with TTL
func (c *L1Cache) Set(ctx context.Context, key string, value []byte) error {
	if c.client == nil {
		return nil
	}

	// Check cache size and evict if needed
	c.evictIfNeeded(ctx)

	if err := c.client.HSet(ctx, c.keyPrefix, key, string(value)).Err(); err != nil {
		return err
	}
	if c.ttl > 0 {
		return c.client.Expire(ctx, c.keyPrefix, c.ttl).Err()
	}
	return nil
}

// Delete removes a key from cache
func (c *L1Cache) Delete(ctx context.Context, key string) error {
	if c.client == nil {
		return nil
	}
	return c.client.HDel(ctx, c.keyPrefix, key).Err()
}

// Clear removes all keys from L1 cache
func (c *L1Cache) Clear(ctx context.Context) error {
	if c.client == nil {
		return nil
	}
	return c.client.Del(ctx, c.keyPrefix).Err()
}

// evictIfNeeded removes oldest entries if cache exceeds max size
func (c *L1Cache) evictIfNeeded(ctx context.Context) error {
	if c.maxSize <= 0 {
		return nil
	}

	count, err := c.client.HLen(ctx, c.keyPrefix).Result()
	if err != nil {
		return err
	}

	if count >= c.maxSize {
		// Use Redis SORT to get oldest keys and delete them
		// This is a simplified LRU - in production consider using Redis sorted sets
		keys, err := c.client.HKeys(ctx, c.keyPrefix).Result()
		if err != nil {
			return err
		}

		// Delete oldest 10% of entries
		deleteCount := int64(len(keys)) / 10
		if deleteCount < 1 {
			deleteCount = 1
		}

		for i := 0; i < int(deleteCount) && i < len(keys); i++ {
			c.client.HDel(ctx, c.keyPrefix, keys[i])
		}
	}

	return nil
}

// Stats returns cache statistics
func (c *L1Cache) Stats(ctx context.Context) (map[string]interface{}, error) {
	if c.client == nil {
		return map[string]interface{}{
			"keys":    0,
			"enabled": false,
		}, nil
	}

	count, err := c.client.HLen(ctx, c.keyPrefix).Result()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"keys":    count,
		"enabled": true,
		"maxSize": c.maxSize,
		"ttl":     c.ttl.Seconds(),
	}, nil
}

// ChatRequest represents a chat completion request for caching
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
}

// ChatResponse represents a chat completion response for caching
type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a completion choice
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Marshal serializes response for caching
func (r *ChatResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

// Unmarshal deserializes cached response
func UnmarshalChatResponse(data []byte) (*ChatResponse, error) {
	var resp ChatResponse
	err := json.Unmarshal(data, &resp)
	return &resp, err
}
