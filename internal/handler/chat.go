package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"llm-gateway/internal/config"
	"llm-gateway/internal/storage"
	"llm-gateway/pkg/errors"

	"github.com/gin-gonic/gin"
)

// ChatCompletion handles chat completion requests 处理聊天完成请求
func ChatCompletion(cfg *config.Config, redisClient *storage.RedisClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse request 解析请求
		var req ChatCompletionRequest
		if err := c.ShouldBindJSON(&req); err != nil { // 绑定JSON请求体到req结构体
			errors.InvalidRequest(err.Error()).JSON(c)
			return
		}

		// Validate request 验证请求
		if err := validateChatRequest(req); err != nil {
			errors.InvalidRequest(err.Error()).JSON(c)
			return
		}

		// 检查缓存是否开启
		if cfg.Cache.Enabled {
			cacheKey := generateCacheKey(req)
			if cached, err := redisClient.HGet(c.Request.Context(), "cache:l1", cacheKey); err == nil && cached != "" { //内存使用更高效 可以方便地批量操作相关的缓存项
				// Cache hit - return cached response // 缓存命中 - 返回缓存响应
				var resp ChatCompletionResponse
				if err := json.Unmarshal([]byte(cached), &resp); err == nil { // 解析缓存数据到resp结构体
					if req.Stream { // 如果请求流模式
						handleStreamingResponse(c, &resp) // 处理流式响应 模拟流式响应 保护用户体验
					} else {
						c.JSON(http.StatusOK, resp) // 处理非流式响应
					}
					return
				}
			}
		}

		// Forward to LLM provider 转发到LLM提供程序
		resp, err := forwardToProvider(c, cfg, req)
		if err != nil {
			errors.InternalError(err.Error()).JSON(c)
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

// ChatCompletionRequest represents the OpenAI chat completion request  表示OpenAI聊天完成请求
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

func generateCacheKey(req ChatCompletionRequest) string {
	// Simple hash based on model + first message content 基于模型 + 第一条消息内容 生成缓存键
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
	c.Header("Content-Type", "text/event-stream; charset=utf-8") // 设置内容类型为事件流
	c.Header("Cache-Control", "no-cache")                        // 禁用缓存
	c.Header("Connection", "keep-alive")                         // 保持连接_alive

	// Simulate streaming by sending chunks 模拟流式响应，按指定大小发送数据块
	content := resp.Choices[0].Message.Content // 获取响应内容
	chunkSize := 20
	for i := 0; i < len(content); i += chunkSize {
		end := i + chunkSize
		if end > len(content) {
			end = len(content)
		}
		chunk := content[i:end] // 提取当前数据块

		c.SSEvent("message", gin.H{ // 发送事件流消息
			"choices": []gin.H{
				{
					"delta": gin.H{"content": chunk},
				},
			},
		})
		c.Writer.Flush()                  // 刷新响应缓冲区，确保客户端立即接收数据
		time.Sleep(10 * time.Millisecond) // 模拟延迟，实际应用中可根据需要调整
	}

	c.SSEvent("done", nil) // 发送完成事件流消息
}
