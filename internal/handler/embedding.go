package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"llm-gateway/internal/config"
	"llm-gateway/internal/middleware"
	gatewayservice "llm-gateway/internal/service"
	"llm-gateway/internal/service/embeddingworker"
	"llm-gateway/internal/storage"
	apierrors "llm-gateway/pkg/errors"
)

// EmbeddingHandler handles embedding requests.
func EmbeddingHandler(cfg *config.Config, workerClient *embeddingworker.Client, postgresClient *storage.PostgresClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		statusCode := http.StatusOK
		errorCode := ""
		totalTokens := 0
		routedModel := ""
		workflowMeta := extractWorkflowRequestContext(c)
		requestedModel := ""

		var req EmbeddingRequest
		defer func() {
			requestModel := strings.TrimSpace(req.Model)
			if requestModel == "" {
				requestModel = "text-embedding-ada-002"
			}
			if strings.TrimSpace(requestedModel) == "" {
				requestedModel = requestModel
			}
			if strings.TrimSpace(routedModel) == "" {
				routedModel = requestModel
			}

			apiKeyID := ""
			if v, ok := c.Get("api_key_id"); ok {
				if asString, ok := v.(string); ok {
					apiKeyID = asString
				}
			}

			latency := time.Since(startedAt)
			gatewayservice.DefaultUsageStats().Record(gatewayservice.RequestMetric{
				Endpoint:    "/v1/embeddings",
				Model:       routedModel,
				StatusCode:  statusCode,
				Latency:     latency,
				CacheHit:    false,
				TotalTokens: totalTokens,
			})
			persistRequestLog(
				postgresClient,
				"/v1/embeddings",
				apiKeyID,
				routedModel,
				statusCode,
				latency,
				false,
				totalTokens,
			)
			gatewayservice.DefaultWorkflowTracer().Record(c.Request.Context(), gatewayservice.WorkflowTrace{
				TraceID:          workflowMeta.TraceID,
				Endpoint:         "/v1/embeddings",
				SessionID:        workflowMeta.SessionID,
				StepID:           workflowMeta.StepID,
				StepType:         workflowMeta.StepType,
				Tool:             workflowMeta.Tool,
				RequestedModel:   requestedModel,
				UpstreamModel:    routedModel,
				UpstreamProvider: "embedding_worker",
				StatusCode:       statusCode,
				LatencyMs:        latency.Milliseconds(),
				CacheHit:         false,
				TotalTokens:      totalTokens,
				ErrorCode:        errorCode,
			})

			middleware.Info("request processed",
				middleware.String("endpoint", "/v1/embeddings"),
				middleware.String("model", requestModel),
				middleware.String("routed_model", routedModel),
				middleware.Int("status", statusCode),
				middleware.Int("latency_ms", int(latency.Milliseconds())),
				middleware.Int("total_tokens", totalTokens),
				middleware.String("cache_hit", strconv.FormatBool(false)),
			)
		}()

		if err := c.ShouldBindJSON(&req); err != nil {
			statusCode = http.StatusBadRequest
			errorCode = string(apierrors.ErrCodeInvalidRequest)
			apierrors.InvalidRequest(err.Error()).JSON(c)
			return
		}
		requestedModel = strings.TrimSpace(req.Model)
		if req.Model == "" {
			req.Model = "text-embedding-ada-002"
		}

		if workerClient == nil {
			statusCode = http.StatusServiceUnavailable
			errorCode = string(apierrors.ErrCodeServiceUnavailable)
			apierrors.ServiceUnavailable("embedding worker is not configured").JSON(c)
			return
		}

		tracer := otel.Tracer("llm-gateway/handler/embedding")
		_, workerSpan := tracer.Start(c.Request.Context(), "embedding_worker.generate")
		workerSpan.SetAttributes(attribute.String("model", req.Model))
		resp, workerErr := workerClient.Generate(c.Request.Context(), embeddingworker.Request{
			Input: req.Input,
			Model: req.Model,
		})
		if workerErr != nil {
			workerSpan.SetStatus(codes.Error, workerErr.Error())
			workerSpan.End()
			mappedErr := mapEmbeddingWorkerError(workerErr)
			statusCode = mappedErr.HTTPStatus()
			errorCode = string(mappedErr.Code)
			mappedErr.JSON(c)
			return
		}
		workerSpan.SetAttributes(
			attribute.String("upstream.provider", "embedding_worker"),
			attribute.String("upstream.model", resp.Model),
			attribute.Int("usage.total_tokens", resp.Usage.TotalTokens),
		)
		workerSpan.SetStatus(codes.Ok, "")
		workerSpan.End()

		statusCode = http.StatusOK
		routedModel = resp.Model
		totalTokens = resp.Usage.TotalTokens
		c.JSON(http.StatusOK, toEmbeddingResponse(resp))
	}
}

// EmbeddingRequest represents an embedding request.
type EmbeddingRequest struct {
	Input interface{} `json:"input"` // string or []string
	Model string      `json:"model"`
}

// EmbeddingResponse represents an embedding response.
type EmbeddingResponse struct {
	Object string       `json:"object"`
	Data   []EmbedEntry `json:"data"`
	Model  string       `json:"model"`
	Usage  Usage        `json:"usage"`
}

// EmbedEntry represents a single embedding.
type EmbedEntry struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

func mapEmbeddingWorkerError(err *embeddingworker.WorkerError) apierrors.GatewayError {
	message := "upstream embedding provider error"
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
		return apierrors.ServiceUnavailable(message)
	default:
		if err.StatusCode >= http.StatusInternalServerError {
			return apierrors.ProviderError(message)
		}
		return apierrors.InternalError(message)
	}
}

func toEmbeddingResponse(resp *embeddingworker.Response) *EmbeddingResponse {
	entries := make([]EmbedEntry, len(resp.Data))
	for i, entry := range resp.Data {
		entries[i] = EmbedEntry{
			Object:    entry.Object,
			Embedding: entry.Embedding,
			Index:     entry.Index,
		}
	}

	return &EmbeddingResponse{
		Object: resp.Object,
		Data:   entries,
		Model:  resp.Model,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
}
