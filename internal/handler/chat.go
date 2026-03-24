package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"llm-gateway/internal/config"
	"llm-gateway/internal/middleware"
	gatewayservice "llm-gateway/internal/service"
	"llm-gateway/internal/service/cache"
	"llm-gateway/internal/service/embeddingworker"
	"llm-gateway/internal/service/provider"
	"llm-gateway/internal/storage"
	apierrors "llm-gateway/pkg/errors"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// ChatCompletion handles chat completion requests with L1/L2 caching.
func ChatCompletion(cfg *config.Config, providerRegistry *provider.Registry, workerClient *embeddingworker.Client, postgresClient *storage.PostgresClient, l1Cache *cache.L1Cache, l2Cache *cache.L2Cache) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		statusCode := http.StatusOK
		cacheHit := false
		promptTokens := 0
		completionTokens := 0
		totalTokens := 0
		routedModel := ""
		upstreamProvider := ""
		errorCode := ""
		workflowMeta := extractWorkflowRequestContext(c)
		requestedModel := ""
		cacheKey := ""
		semanticVector := []float64{}

		var req ChatCompletionRequest
		defer func() {
			requestModel := strings.TrimSpace(req.Model)
			if requestModel == "" {
				requestModel = "unknown"
			}
			if strings.TrimSpace(requestedModel) == "" {
				requestedModel = requestModel
			}
			apiKeyID := ""
			if v, ok := c.Get("api_key_id"); ok {
				if asString, ok := v.(string); ok {
					apiKeyID = asString
				}
			}

			latency := time.Since(startedAt)
			routed := routedModelOrRequest(routedModel, requestModel)
			gatewayservice.DefaultUsageStats().Record(gatewayservice.RequestMetric{
				Endpoint:    "/v1/chat/completions",
				Model:       routed,
				StatusCode:  statusCode,
				Latency:     latency,
				CacheHit:    cacheHit,
				TotalTokens: totalTokens,
			})
			persistRequestLog(
				postgresClient,
				"/v1/chat/completions",
				apiKeyID,
				routed,
				statusCode,
				latency,
				cacheHit,
				totalTokens,
			)
			gatewayservice.DefaultWorkflowTracer().Record(c.Request.Context(), gatewayservice.WorkflowTrace{
				TraceID:          workflowMeta.TraceID,
				Endpoint:         "/v1/chat/completions",
				SessionID:        workflowMeta.SessionID,
				StepID:           workflowMeta.StepID,
				StepType:         workflowMeta.StepType,
				Tool:             workflowMeta.Tool,
				RequestedModel:   requestedModel,
				UpstreamModel:    routed,
				UpstreamProvider: upstreamProvider,
				StatusCode:       statusCode,
				LatencyMs:        latency.Milliseconds(),
				CacheHit:         cacheHit,
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      totalTokens,
				ErrorCode:        errorCode,
			})

			middleware.Info("request processed",
				middleware.String("endpoint", "/v1/chat/completions"),
				middleware.String("model", requestModel),
				middleware.String("routed_model", routed),
				middleware.Int("status", statusCode),
				middleware.Int("latency_ms", int(latency.Milliseconds())),
				middleware.Int("total_tokens", totalTokens),
				middleware.String("cache_hit", strconv.FormatBool(cacheHit)),
			)
		}()

		if err := c.ShouldBindJSON(&req); err != nil {
			statusCode = http.StatusBadRequest
			errorCode = string(apierrors.ErrCodeInvalidRequest)
			apierrors.InvalidRequest(err.Error()).JSON(c)
			return
		}
		requestedModel = strings.TrimSpace(req.Model)
		applyWorkflowRoutePolicy(cfg, &req, workflowMeta.StepType)

		if err := validateChatRequest(req); err != nil {
			statusCode = http.StatusBadRequest
			errorCode = string(apierrors.ErrCodeInvalidRequest)
			apierrors.InvalidRequest(err.Error()).JSON(c)
			return
		}

		ctx := c.Request.Context()
		tracer := otel.Tracer("llm-gateway/handler/chat")
		params := map[string]interface{}{
			"temperature": req.Temperature,
			"max_tokens":  req.MaxTokens,
			"top_p":       req.TopP,
			"stop":        req.Stop,
		}
		cacheKey = cache.GenerateCacheKey(req.Model, toCacheMessages(req.Messages), params)

		if cfg.Cache.Enabled && l1Cache != nil {
			_, l1Span := tracer.Start(ctx, "cache.l1.lookup")
			l1Span.SetAttributes(attribute.String("model", req.Model))
			if cached, err := l1Cache.Get(ctx, cacheKey); err == nil && cached != nil {
				if data, err := cache.UnmarshalChatResponse(cached); err == nil {
					l1Span.SetAttributes(attribute.Bool("cache.hit", true))
					l1Span.SetStatus(codes.Ok, "")
					l1Span.End()
					statusCode = http.StatusOK
					cacheHit = true
					routedModel = data.Model
					upstreamProvider = "cache_l1"
					promptTokens = data.Usage.PromptTokens
					completionTokens = data.Usage.CompletionTokens
					totalTokens = data.Usage.TotalTokens
					if req.Stream {
						handleStreamingResponse(c, toChatResponse(data))
					} else {
						c.JSON(http.StatusOK, toChatResponse(data))
					}
					return
				}
			}
			l1Span.SetAttributes(attribute.Bool("cache.hit", false))
			l1Span.End()
		}

		if cfg.Cache.Enabled && l2Cache != nil && workerClient != nil && workerClient.Healthy(ctx) {
			_, l2Span := tracer.Start(ctx, "cache.l2.lookup")
			l2Span.SetAttributes(attribute.String("model", req.Model))
			vector, err := fetchSemanticVector(ctx, workerClient, req.Messages)
			if err == nil && len(vector) > 0 {
				semanticVector = vector
				if hit, err := l2Cache.Search(ctx, vector, req.Model); err == nil && hit != nil {
					var cachedResp ChatCompletionResponse
					if err := json.Unmarshal([]byte(hit.Response), &cachedResp); err == nil {
						l2Span.SetAttributes(
							attribute.Bool("cache.hit", true),
							attribute.Float64("cache.similarity", hit.Similarity),
						)
						l2Span.SetStatus(codes.Ok, "")
						l2Span.End()
						statusCode = http.StatusOK
						cacheHit = true
						routedModel = cachedResp.Model
						upstreamProvider = "cache_l2"
						promptTokens = cachedResp.Usage.PromptTokens
						completionTokens = cachedResp.Usage.CompletionTokens
						totalTokens = cachedResp.Usage.TotalTokens
						if req.Stream {
							handleStreamingResponse(c, &cachedResp)
						} else {
							c.JSON(http.StatusOK, &cachedResp)
						}
						return
					}
				}
			}
			l2Span.SetAttributes(attribute.Bool("cache.hit", false))
			if err != nil {
				l2Span.SetStatus(codes.Error, err.Error())
			}
			l2Span.End()
		}

		_, providerSpan := tracer.Start(ctx, "provider.call")
		providerSpan.SetAttributes(attribute.String("model", req.Model))
		resp, providerName, gatewayErr := forwardToProvider(ctx, providerRegistry, req)
		if gatewayErr != nil {
			providerSpan.SetStatus(codes.Error, gatewayErr.Message)
			providerSpan.End()
			statusCode = gatewayErr.HTTPStatus()
			errorCode = string(gatewayErr.Code)
			gatewayErr.JSON(c)
			return
		}
		providerSpan.SetAttributes(
			attribute.String("upstream.provider", providerName),
			attribute.String("upstream.model", resp.Model),
			attribute.Int("usage.total_tokens", resp.Usage.TotalTokens),
		)
		providerSpan.SetStatus(codes.Ok, "")
		providerSpan.End()
		routedModel = resp.Model
		upstreamProvider = providerName
		promptTokens = resp.Usage.PromptTokens
		completionTokens = resp.Usage.CompletionTokens
		totalTokens = resp.Usage.TotalTokens
		statusCode = http.StatusOK

		if cfg.Cache.Enabled && l1Cache != nil {
			cacheResp := toCacheResponse(resp)
			if data, err := cacheResp.Marshal(); err == nil {
				_ = l1Cache.Set(ctx, cacheKey, data)
			}
		}

		if cfg.Cache.Enabled && l2Cache != nil && workerClient != nil {
			if len(semanticVector) == 0 && workerClient.Healthy(ctx) {
				if vector, err := fetchSemanticVector(ctx, workerClient, req.Messages); err == nil {
					semanticVector = vector
				}
			}

			if len(semanticVector) > 0 {
				if payload, err := json.Marshal(resp); err == nil {
					_ = l2Cache.Store(ctx, cacheKey, semanticVector, string(payload), req.Model)
				}
			}
		}

		if req.Stream {
			handleStreamingResponse(c, resp)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func routedModelOrRequest(routedModel, requestModel string) string {
	if strings.TrimSpace(routedModel) != "" {
		return routedModel
	}
	return requestModel
}

type workflowRequestContext struct {
	TraceID   string
	SessionID string
	StepID    string
	StepType  string
	Tool      string
}

func extractWorkflowRequestContext(c *gin.Context) workflowRequestContext {
	return workflowRequestContext{
		TraceID:   strings.TrimSpace(c.GetHeader("X-Workflow-Trace-Id")),
		SessionID: strings.TrimSpace(c.GetHeader("X-Workflow-Session")),
		StepID:    strings.TrimSpace(c.GetHeader("X-Workflow-Step")),
		StepType:  normalizeWorkflowStepType(c.GetHeader("X-Workflow-Phase")),
		Tool:      strings.TrimSpace(c.GetHeader("X-Workflow-Tool")),
	}
}

func normalizeWorkflowStepType(raw string) string {
	stepType := strings.ToLower(strings.TrimSpace(raw))
	switch stepType {
	case "planning", "execution", "summarization":
		return stepType
	default:
		return ""
	}
}

func applyWorkflowRoutePolicy(cfg *config.Config, req *ChatCompletionRequest, stepType string) {
	if cfg == nil || req == nil {
		return
	}
	stepType = normalizeWorkflowStepType(stepType)
	if stepType == "" {
		return
	}
	if cfg.Workflow.RoutePolicies == nil {
		return
	}

	model, ok := cfg.Workflow.RoutePolicies[stepType]
	if !ok {
		return
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return
	}

	req.Model = model
}

// ChatCompletionRequest represents the OpenAI chat completion request.
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	TopP        float64       `json:"top_p,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
}

// ChatMessage represents a chat message.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// ChatCompletionResponse represents the OpenAI chat completion response.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a completion choice.
type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// Usage represents token usage.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func validateChatRequest(req ChatCompletionRequest) error {
	if strings.TrimSpace(req.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if len(req.Messages) == 0 {
		return fmt.Errorf("messages are required")
	}
	for i, msg := range req.Messages {
		if strings.TrimSpace(msg.Role) == "" {
			return fmt.Errorf("messages[%d].role is required", i)
		}
		if strings.TrimSpace(msg.Content) == "" {
			return fmt.Errorf("messages[%d].content is required", i)
		}
		switch msg.Role {
		case "system", "user", "assistant", "tool":
		default:
			return fmt.Errorf("messages[%d].role is invalid", i)
		}
	}
	if req.Temperature < 0 || req.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}
	if req.TopP < 0 || req.TopP > 1 {
		return fmt.Errorf("top_p must be between 0 and 1")
	}
	if req.MaxTokens < 0 {
		return fmt.Errorf("max_tokens must be greater than or equal to 0")
	}
	if len(req.Stop) > 4 {
		return fmt.Errorf("stop must contain at most 4 sequences")
	}
	return nil
}

func forwardToProvider(ctx context.Context, providerRegistry *provider.Registry, req ChatCompletionRequest) (*ChatCompletionResponse, string, *apierrors.GatewayError) {
	if providerRegistry == nil {
		err := apierrors.ServiceUnavailable("provider registry is not initialized")
		return nil, "", &err
	}

	resp, upstreamErr := providerRegistry.ChatCompletion(ctx, toProviderRequest(req))
	if upstreamErr != nil {
		mapped := mapUpstreamError(upstreamErr)
		return nil, "", &mapped
	}
	return toHandlerResponse(resp), strings.TrimSpace(resp.Provider), nil
}

func mapUpstreamError(err *provider.UpstreamError) apierrors.GatewayError {
	message := "upstream provider error"
	if err != nil && err.Message != "" {
		message = err.Message
	}

	if err == nil {
		return apierrors.ProviderError(message)
	}

	switch err.StatusCode {
	case http.StatusBadRequest:
		return apierrors.InvalidRequest(message)
	case http.StatusUnauthorized:
		return apierrors.InvalidAPIKey(message)
	case http.StatusForbidden:
		return apierrors.KeyDisabled(message)
	case http.StatusTooManyRequests:
		return apierrors.RateLimitExceeded(message)
	case http.StatusBadGateway:
		return apierrors.BadGateway(message)
	case http.StatusServiceUnavailable:
		if strings.Contains(strings.ToLower(message), "circuit-open") {
			return apierrors.CircuitOpen(message)
		}
		return apierrors.ModelOverloaded(message)
	case http.StatusGatewayTimeout:
		return apierrors.ServiceUnavailable(message)
	}

	if err.StatusCode >= http.StatusInternalServerError {
		return apierrors.ProviderError(message)
	}

	return apierrors.InternalError(message)
}

func toProviderRequest(req ChatCompletionRequest) provider.ChatCompletionRequest {
	msgs := make([]provider.ChatMessage, len(req.Messages))
	for i, msg := range req.Messages {
		msgs[i] = provider.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
			Name:    msg.Name,
		}
	}

	return provider.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    msgs,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		TopP:        req.TopP,
		Stream:      req.Stream,
		Stop:        req.Stop,
	}
}

func toHandlerResponse(resp *provider.ChatCompletionResponse) *ChatCompletionResponse {
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
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
}

func handleStreamingResponse(c *gin.Context, resp *ChatCompletionResponse) {
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	if resp == nil || len(resp.Choices) == 0 {
		c.SSEvent("done", nil)
		return
	}

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

func fetchSemanticVector(ctx context.Context, workerClient *embeddingworker.Client, messages []ChatMessage) ([]float64, error) {
	if workerClient == nil {
		return nil, fmt.Errorf("embedding worker is not configured")
	}

	prompt := buildSemanticPrompt(messages)
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("semantic prompt is empty")
	}

	resp, err := workerClient.Generate(ctx, embeddingworker.Request{
		Model: "text-embedding-ada-002",
		Input: prompt,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.Data) == 0 || len(resp.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embedding vector is empty")
	}
	return resp.Data[0].Embedding, nil
}

func buildSemanticPrompt(messages []ChatMessage) string {
	if len(messages) == 0 {
		return ""
	}

	var b strings.Builder
	for _, msg := range messages {
		role := strings.TrimSpace(msg.Role)
		content := strings.TrimSpace(msg.Content)
		if role == "" && content == "" {
			continue
		}
		if role == "" {
			role = "unknown"
		}
		b.WriteString(role)
		b.WriteString(": ")
		b.WriteString(content)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

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
			Index: c.Index,
			Message: cache.Message{
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
			TotalTokens:      resp.Usage.TotalTokens,
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
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
}
