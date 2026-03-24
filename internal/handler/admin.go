package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"llm-gateway/internal/config"
	"llm-gateway/internal/middleware"
	"llm-gateway/internal/service"
	"llm-gateway/internal/storage"
	"llm-gateway/pkg/errors"
)

// AdminAuth validates admin requests
func AdminAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Simple admin key check - in production use proper auth
		adminKey := c.GetHeader("X-Admin-Key")
		if adminKey == "" {
			errors.InvalidRequest("Missing admin key").JSON(c)
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
			errors.InvalidRequest(err.Error()).JSON(c)
			return
		}

		// Generate new API key
		apiKey := "sk-" + generateRandomString(32)
		keyHash := hashKey(apiKey)

		// Save to database
		key, err := pg.CreateAPIKey(c.Request.Context(), keyHash, req.Name, req.RateLimit)
		if err != nil {
			errors.InternalError("Failed to create API key").JSON(c)
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
			errors.InternalError("Failed to list API keys").JSON(c)
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
			errors.InvalidRequest("Key ID is required").JSON(c)
			return
		}

		if err := pg.DeleteAPIKey(c.Request.Context(), id); err != nil {
			errors.InternalError("Failed to delete API key").JSON(c)
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "API key deleted"})
	}
}

// GetStats returns usage statistics
func GetStats(pg *storage.PostgresClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		if pg != nil {
			snapshot, err := pg.GetUsageSnapshot(c.Request.Context())
			if err == nil {
				c.JSON(http.StatusOK, snapshot)
				return
			}
			middleware.Warn("failed to read stats from postgres, falling back to in-memory stats", middleware.Err(err))
		}

		snapshot := service.DefaultUsageStats().Snapshot()
		c.JSON(http.StatusOK, snapshot)
	}
}

// GetWorkflowSummary returns one workflow session summary from in-memory tracer.
func GetWorkflowSummary() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := strings.TrimSpace(c.Param("session_id"))
		if sessionID == "" {
			errors.InvalidRequest("session_id is required").JSON(c)
			return
		}

		summary, ok := service.DefaultWorkflowTracer().GetSummary(sessionID)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"message": "workflow session not found",
					"type":    "invalid_request_error",
					"code":    "invalid_request",
				},
			})
			return
		}

		c.JSON(http.StatusOK, summary)
	}
}

// ListWorkflowSummaries lists recent workflow summaries.
func ListWorkflowSummaries() gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := 20
		if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				limit = parsed
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"object":    "list",
			"summaries": service.DefaultWorkflowTracer().ListSummaries(limit),
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
