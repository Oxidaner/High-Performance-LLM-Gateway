package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"
)

// L2Cache implements semantic caching using vector similarity
type L2Cache struct {
	client            *redis.Client
	similarityThreshold float64
	ttl              time.Duration
	maxSize          int64
	keyPrefix        string
	vectorDim        int
}

// L2CacheConfig holds L2 cache configuration
type L2CacheConfig struct {
	Enabled            bool
	SimilarityThreshold float64 // 0.0 - 1.0, typically 0.95
	TTL               int      // seconds
	MaxSize           int64
	KeyPrefix         string
	VectorDim         int      // embedding dimension
}

// NewL2Cache creates a new L2 cache instance
func NewL2Cache(client *redis.Client, cfg L2CacheConfig) *L2Cache {
	return &L2Cache{
		client:              client,
		similarityThreshold: cfg.SimilarityThreshold,
		ttl:                 time.Duration(cfg.TTL) * time.Second,
		maxSize:             cfg.MaxSize,
		keyPrefix:           cfg.KeyPrefix,
		vectorDim:           cfg.VectorDim,
	}
}

// VectorEntry represents a cached embedding with its response
type VectorEntry struct {
	Vector   []float64 `json:"vector"`
	Response string    `json:"response"`
	Model    string    `json:"model"`
	Created  int64     `json:"created"`
}

// SearchResult contains similar cached response
type SearchResult struct {
	Response   string  `json:"response"`
	Similarity float64 `json:"similarity"`
	Key        string  `json:"key"`
}

// Search finds similar responses based on vector embedding
func (c *L2Cache) Search(ctx context.Context, queryVector []float64, model string) (*SearchResult, error) {
	if c.client == nil {
		return nil, nil
	}

	// Get all entries for the model
	keys, err := c.client.Keys(ctx, c.keyPrefix+":*").Result()
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return nil, nil
	}

	var bestMatch *SearchResult
	var bestSimilarity float64

	for _, key := range keys {
		// Get stored vector
		data, err := c.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var entry VectorEntry
		if err := json.Unmarshal([]byte(data), &entry); err != nil {
			continue
		}

		// Skip if model doesn't match
		if model != "" && entry.Model != model {
			continue
		}

		// Calculate cosine similarity
		similarity := cosineSimilarity(queryVector, entry.Vector)

		if similarity >= c.similarityThreshold {
			if bestMatch == nil || similarity > bestSimilarity {
				bestSimilarity = similarity
				bestMatch = &SearchResult{
					Response:   entry.Response,
					Similarity: similarity,
					Key:        key,
				}
			}
		}
	}

	return bestMatch, nil
}

// Store stores a response with its embedding vector
func (c *L2Cache) Store(ctx context.Context, key string, vector []float64, response string, model string) error {
	if c.client == nil {
		return nil
	}

	// Check and evict if needed
	c.evictIfNeeded(ctx)

	entry := VectorEntry{
		Vector:   vector,
		Response: response,
		Model:    model,
		Created:  time.Now().Unix(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	// Use vector key format: vector:{hash}
	vectorKey := c.keyPrefix + ":" + key

	return c.client.Set(ctx, vectorKey, data, c.ttl).Err()
}

// Delete removes a vector entry from cache
func (c *L2Cache) Delete(ctx context.Context, key string) error {
	if c.client == nil {
		return nil
	}
	vectorKey := c.keyPrefix + ":" + key
	return c.client.Del(ctx, vectorKey).Err()
}

// Clear removes all L2 cache entries
func (c *L2Cache) Clear(ctx context.Context) error {
	if c.client == nil {
		return nil
	}

	keys, err := c.client.Keys(ctx, c.keyPrefix+":*").Result()
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		return c.client.Del(ctx, keys...).Err()
	}
	return nil
}

// evictIfNeeded removes oldest entries if cache exceeds max size
func (c *L2Cache) evictIfNeeded(ctx context.Context) error {
	if c.maxSize <= 0 {
		return nil
	}

	keys, err := c.client.Keys(ctx, c.keyPrefix+":*").Result()
	if err != nil {
		return err
	}

	if int64(len(keys)) >= c.maxSize {
		// Sort by creation time and delete oldest 10%
		var entries []struct {
			Key     string
			Entry   VectorEntry
			Created int64
		}

		for _, key := range keys {
			data, err := c.client.Get(ctx, key).Result()
			if err != nil {
				continue
			}
			var entry VectorEntry
			if err := json.Unmarshal([]byte(data), &entry); err != nil {
				continue
			}
			entries = append(entries, struct {
				Key     string
				Entry   VectorEntry
				Created int64
			}{key, entry, entry.Created})
		}

		// Sort by created time
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Created < entries[j].Created
		})

		// Delete oldest 10%
		deleteCount := len(entries) / 10
		if deleteCount < 1 {
			deleteCount = 1
		}

		keysToDelete := make([]string, deleteCount)
		for i := 0; i < deleteCount; i++ {
			keysToDelete[i] = entries[i].Key
		}

		c.client.Del(ctx, keysToDelete...)
	}

	return nil
}

// Stats returns cache statistics
func (c *L2Cache) Stats(ctx context.Context) (map[string]interface{}, error) {
	if c.client == nil {
		return map[string]interface{}{
			"keys":               0,
			"enabled":            false,
			"similarityThreshold": c.similarityThreshold,
		}, nil
	}

	keys, err := c.client.Keys(ctx, c.keyPrefix+":*").Result()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"keys":               len(keys),
		"enabled":            true,
		"similarityThreshold": c.similarityThreshold,
		"maxSize":           c.maxSize,
		"ttl":               c.ttl.Seconds(),
	}, nil
}

// cosineSimilarity calculates cosine similarity between two vectors
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// EmbeddingRequest represents request to get embedding from worker
type EmbeddingRequest struct {
	Input string `json:"input"`
	Model string `json:"model"`
}

// EmbeddingResponse represents embedding from worker
type EmbeddingResponse struct {
	Object    string          `json:"object"`
	Data      []EmbeddingData `json:"data"`
	Model     string          `json:"model"`
	Usage     Usage           `json:"usage"`
}

// EmbeddingData contains the embedding vector
type EmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// NormalizeVector normalizes a vector to unit length
func NormalizeVector(v []float64) []float64 {
	var norm float64
	for _, val := range v {
		norm += val * val
	}
	norm = math.Sqrt(norm)

	if norm == 0 {
		return v
	}

	normalized := make([]float64, len(v))
	for i, val := range v {
		normalized[i] = val / norm
	}

	return normalized
}

// FormatVectorKey creates a Redis key for vector storage
func FormatVectorKey(hash string) string {
	return fmt.Sprintf("l2:vector:%s", hash)
}
