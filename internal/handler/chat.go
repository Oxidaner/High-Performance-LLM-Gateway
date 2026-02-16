package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"llm-gateway/internal/config"
	"llm-gateway/internal/storage"
)

// ChatCompletion handles chat completion requests
func ChatCompletion(cfg *config.Config, redisClient *storage.RedisClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse request
		var req ChatCompletionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": err.Error(),
					"type":   "invalid_request_error",
				},
			})
			return
		}

		// Validate request
		if err := validateChatRequest(req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": err.Error(),
					"type":   "invalid_request_error",
				},
			})
			return
		}

		// Check L1 cache
		if cfg.Cache.Enabled {
			cacheKey := generateCacheKey(req)
			if cached, err := redisClient.HGet(c.Request.Context(), "cache:l1", cacheKey); err == nil && cached != "" {
				// Cache hit - return cached response
				var resp ChatCompletionResponse
				if err := json.Unmarshal([]byte(cached), &resp); err == nil {
					if req.Stream {
						handleStreamingResponse(c, &resp)
					} else {
						c.JSON(http.StatusOK, resp)
					}
					return
				}
			}
		}

		// Forward to LLM provider
		resp, err := forwardToProvider(c, cfg, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"message": err.Error(),
					"type":   "server_error",
				},
			})
			return
		}

		// Cache response
		if cfg.Cache.Enabled {
			cacheKey := generateCacheKey(req)
			if data, err := json.Marshal(resp); err == nil {
				redisClient.HSet(c.Request.Context(), "cache:l1", cacheKey, string(data))
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
	Stop        []string     `json:"stop,omitempty"`
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
	TotalTokens     int `json:"total_tokens"`
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

func generateCacheKey(req ChatCompletionRequest) string {
	// Simple hash based on model + first message content
	data := fmt.Sprintf("%s:%s", req.Model, req.Messages[0].Content)
	return fmt.Sprintf("%x", []byte(data))
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
	if model == "gpt-4" || model == "gpt-3.5-turbo" {
		return "openai"
	}
	if model == "claude-3-haiku" || model == "claude-3-sonnet" {
		return "anthropic"
	}
	return "openai"
}

func handleStreamingResponse(c *gin.Context, resp *ChatCompletionResponse) {
	c.Header("Content-Type", "text/event-stream")
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
