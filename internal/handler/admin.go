package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"llm-gateway/internal/config"
	"llm-gateway/internal/storage"
)

// AdminAuth validates admin requests
func AdminAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Simple admin key check - in production use proper auth
		adminKey := c.GetHeader("X-Admin-Key")
		if adminKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "Missing admin key",
					"type":   "invalid_request_error",
				},
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// CreateAPIKey creates a new API key
func CreateAPIKey(pg *storage.PostgresClient, redisClient *storage.RedisClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateKeyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": err.Error(),
					"type":   "invalid_request_error",
				},
			})
			return
		}

		// Generate new API key
		apiKey := "sk-" + generateRandomString(32)
		keyHash := hashKey(apiKey)

		// Save to database
		key, err := pg.CreateAPIKey(c.Request.Context(), keyHash, req.Name, req.RateLimit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"message": "Failed to create API key",
					"type":   "server_error",
				},
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"id":         key.ID,
			"key":        apiKey,
			"name":       key.Name,
			"rate_limit": key.RateLimit,
			"is_active":  key.IsActive,
		})
	}
}

// ListAPIKeys lists all API keys
func ListAPIKeys(pg *storage.PostgresClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		keys, err := pg.ListAPIKeys(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"message": "Failed to list API keys",
					"type":   "server_error",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{"keys": keys})
	}
}

// DeleteAPIKey deletes an API key
func DeleteAPIKey(pg *storage.PostgresClient, redisClient *storage.RedisClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "Key ID is required",
					"type":   "invalid_request_error",
				},
			})
			return
		}

		if err := pg.DeleteAPIKey(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"message": "Failed to delete API key",
					"type":   "server_error",
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "API key deleted"})
	}
}

// GetStats returns usage statistics
func GetStats(pg *storage.PostgresClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Placeholder - would query actual stats from database
		c.JSON(http.StatusOK, gin.H{
			"total_requests":   0,
			"total_tokens":     0,
			"total_cost":       0,
			"cache_hit_rate":   0,
		})
	}
}

// ListModels lists available models
func ListModels(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		models := make([]gin.H, len(cfg.Models))
		for i, m := range cfg.Models {
			models[i] = gin.H{
				"id":         m.Name,
				"object":     "model",
				"created":    0,
				"owned_by":   m.Provider,
				"permission": []gin.H{},
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"object": "list",
			"data":   models,
		})
	}
}

type CreateKeyRequest struct {
	Name      string `json:"name"`
	RateLimit int    `json:"rate_limit"`
}

func generateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}

func hashKey(key string) string {
	// Simple hash - in production use proper hashing
	return strings.ToLower(key)
}
