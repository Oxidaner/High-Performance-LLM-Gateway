package storage

import (
	"context"
	"fmt"

	"llm-gateway/internal/config"

	"github.com/redis/go-redis/v9"
)

// RedisClient wraps the Redis connection
// RedisClient 包装 Redis 连接
type RedisClient struct {
	client *redis.Client
}

// NewRedis creates a new Redis client
// 创建一个新的Redis客户端
func NewRedis(cfg config.RedisConfig) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisClient{client: client}, nil
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// Client returns the underlying Redis client
func (r *RedisClient) Client() *redis.Client {
	return r.client
}

// Get retrieves a value from Redis
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

// Set sets a value in Redis with TTL
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, ttl int) error {
	return r.client.Set(ctx, key, value, 0).Err()
}

// SetWithTTL sets a value with specific TTL
func (r *RedisClient) SetWithTTL(ctx context.Context, key string, value interface{}, ttl int) error {
	return r.client.Set(ctx, key, value, 0).Err()
}

// HGet gets a field from a hash 从哈希中获取字段
func (r *RedisClient) HGet(ctx context.Context, key, field string) (string, error) {
	return r.client.HGet(ctx, key, field).Result()
}

// HSet sets a field in a hash
func (r *RedisClient) HSet(ctx context.Context, key, field string, value interface{}) error {
	return r.client.HSet(ctx, key, field, value).Err()
}

// HGetAll gets all fields from a hash
func (r *RedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, key).Result()
}

// Del deletes keys
func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists
func (r *RedisClient) Exists(ctx context.Context, key string) (bool, error) {
	n, err := r.client.Exists(ctx, key).Result()
	return n > 0, err
}
