package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"llm-gateway/internal/config"
	"llm-gateway/internal/service/cache"
	"llm-gateway/internal/storage"
	"llm-gateway/pkg/errors"

	"github.com/gin-gonic/gin"
)

// ChatCompletion handles chat completion requests with L1/L2 caching
func ChatCompletion(cfg *config.Config, redisClient *storage.RedisClient, l1Cache *cache.L1Cache, l2Cache *cache.L2Cache) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse request
		var req ChatCompletionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			errors.InvalidRequest(err.Error()).JSON(c)
			return
		}

		// Validate request
		if err := validateChatRequest(req); err != nil {
			errors.InvalidRequest(err.Error()).JSON(c)
			return
		}

		ctx := c.Request.Context()

		// ===== L1 精确缓存 =====
		if cfg.Cache.Enabled && l1Cache != nil {
			// Build params for cache key
			params := map[string]interface{}{
				"temperature": req.Temperature,
				"max_tokens":  req.MaxTokens,
				"top_p":       req.TopP,
				"stop":        req.Stop,
			}
			cacheKey := cache.GenerateCacheKey(req.Model, toCacheMessages(req.Messages), params)

			// L1 cache lookup
			if cached, err := l1Cache.Get(ctx, cacheKey); err == nil && cached != nil {
				if data, err := cache.UnmarshalChatResponse(cached); err == nil {
					if req.Stream {
						handleStreamingResponse(c, toChatResponse(data))
					} else {
						c.JSON(http.StatusOK, toChatResponse(data))
					}
					return
				}
			}
		}

		// ===== L2 语义缓存 =====
		// Note: L2 requires embedding calculation, implemented separately
		// For now, skip L2 and go directly to provider

		// ===== Forward to LLM provider =====
		resp, err := forwardToProvider(c, cfg, req)
		if err != nil {
			errors.InternalError(err.Error()).JSON(c)
			return
		}

		// ===== Cache response =====
		if cfg.Cache.Enabled && l1Cache != nil {
			params := map[string]interface{}{
				"temperature": req.Temperature,
				"max_tokens":  req.MaxTokens,
				"top_p":       req.TopP,
				"stop":        req.Stop,
			}
			cacheKey := cache.GenerateCacheKey(req.Model, toCacheMessages(req.Messages), params)

			// Convert to cache response
			cacheResp := toCacheResponse(resp)
			if data, err := cacheResp.Marshal(); err == nil {
				l1Cache.Set(ctx, cacheKey, data)
			}
		}

		// Return response
		if req.Stream {
			handleStreamingResponse(c, resp)
		} else {
			c.JSON(http.StatusOK, resp)
		}
	}
}

// ChatCompletionRequest represents the OpenAI chat completion request
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	TopP        float64       `json:"top_p,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// ChatCompletionResponse represents the OpenAI chat completion response
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a completion choice
type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func validateChatRequest(req ChatCompletionRequest) error {
	if req.Model == "" {
		return fmt.Errorf("model is required")
	}
	if len(req.Messages) == 0 {
		return fmt.Errorf("messages are required")
	}
	return nil
}

func forwardToProvider(c *gin.Context, cfg *config.Config, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	// Find provider configuration
	var providerCfg config.ProviderConfig
	switch getProvider(req.Model) {
	case "openai":
		providerCfg = cfg.Providers.OpenAI
	case "anthropic":
		providerCfg = cfg.Providers.Anthropic
	case "minimax":
		providerCfg = cfg.Providers.MiniMax
	}

	if providerCfg.APIKey == "" {
		return nil, fmt.Errorf("provider not configured")
	}

	// Build request to upstream provider
	url := providerCfg.BaseURL + "/chat/completions"
	jsonData, _ := json.Marshal(req)

	httpReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+providerCfg.APIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream provider error: %s", string(body))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, err
	}

	return &chatResp, nil
}

func getProvider(model string) string {
	// Simple provider detection
	if model == "gpt-4" || model == "gpt-3.5-turbo" || model == "gpt-4o" || model == "gpt-4o-mini" {
		return "openai"
	}
	if model == "claude-3-haiku" || model == "claude-3-sonnet" || model == "claude-3-opus" {
		return "anthropic"
	}
	if model == "abab6.5s-chat" || model == "abab6.5g-chat" {
		return "minimax"
	}
	return "openai"
}

func handleStreamingResponse(c *gin.Context, resp *ChatCompletionResponse) {
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// Simulate streaming by sending chunks
	content := resp.Choices[0].Message.Content
	chunkSize := 20
	for i := 0; i < len(content); i += chunkSize {
		end := i + chunkSize
		if end > len(content) {
			end = len(content)
		}
		chunk := content[i:end]

		c.SSEvent("message", gin.H{
			"choices": []gin.H{
				{
					"delta": gin.H{"content": chunk},
				},
			},
		})
		c.Writer.Flush()
		time.Sleep(10 * time.Millisecond)
	}

	c.SSEvent("done", nil)
}

// Helper functions for cache conversion

func toCacheMessages(msgs []ChatMessage) []cache.Message {
	result := make([]cache.Message, len(msgs))
	for i, msg := range msgs {
		result[i] = cache.Message{
			Role:    msg.Role,
			Content: msg.Content,
			Name:    msg.Name,
		}
	}
	return result
}

func toCacheResponse(resp *ChatCompletionResponse) *cache.ChatResponse {
	choices := make([]cache.Choice, len(resp.Choices))
	for i, c := range resp.Choices {
		choices[i] = cache.Choice{
			Index:        c.Index,
			Message:      cache.Message{
				Role:    c.Message.Role,
				Content: c.Message.Content,
				Name:    c.Message.Name,
			},
			FinishReason: c.FinishReason,
		}
	}

	return &cache.ChatResponse{
		ID:      resp.ID,
		Object:  resp.Object,
		Created: resp.Created,
		Model:   resp.Model,
		Choices: choices,
		Usage: cache.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:     resp.Usage.TotalTokens,
		},
	}
}

func toChatResponse(resp *cache.ChatResponse) *ChatCompletionResponse {
	choices := make([]Choice, len(resp.Choices))
	for i, c := range resp.Choices {
		choices[i] = Choice{
			Index: c.Index,
			Message: ChatMessage{
				Role:    c.Message.Role,
				Content: c.Message.Content,
				Name:    c.Message.Name,
			},
			FinishReason: c.FinishReason,
		}
	}

	return &ChatCompletionResponse{
		ID:      resp.ID,
		Object:  resp.Object,
		Created: resp.Created,
		Model:   resp.Model,
		Choices: choices,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:     resp.Usage.TotalTokens,
		},
	}
}
