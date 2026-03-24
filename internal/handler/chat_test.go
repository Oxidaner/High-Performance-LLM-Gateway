package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"llm-gateway/internal/config"
	"llm-gateway/internal/service/cache"
	"llm-gateway/internal/service/embeddingworker"
	"llm-gateway/internal/service/provider"
	apierrors "llm-gateway/pkg/errors"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func TestChatCompletion_RequestValidation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	router := gin.New()
	router.POST("/v1/chat/completions", ChatCompletion(cfg, nil, nil, nil, nil, nil))

	testCases := []struct {
		name     string
		body     string
		wantCode apierrors.ErrorCode
	}{
		{
			name:     "missing model",
			body:     `{"messages":[{"role":"user","content":"hello"}]}`,
			wantCode: apierrors.ErrCodeInvalidRequest,
		},
		{
			name:     "missing messages",
			body:     `{"model":"gpt-4","messages":[]}`,
			wantCode: apierrors.ErrCodeInvalidRequest,
		},
		{
			name:     "invalid message role",
			body:     `{"model":"gpt-4","messages":[{"role":"bad-role","content":"hello"}]}`,
			wantCode: apierrors.ErrCodeInvalidRequest,
		},
		{
			name:     "invalid top_p",
			body:     `{"model":"gpt-4","top_p":1.5,"messages":[{"role":"user","content":"hello"}]}`,
			wantCode: apierrors.ErrCodeInvalidRequest,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rec.Code, rec.Body.String())
			}

			var resp apierrors.ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse error response: %v", err)
			}
			if resp.Error.Code != tc.wantCode {
				t.Fatalf("expected code %q, got %q", tc.wantCode, resp.Error.Code)
			}
		})
	}
}

func TestChatCompletion_L2CacheHitWithoutProviderRegistry(t *testing.T) {
	gin.SetMode(gin.TestMode)

	l2Cache, cleanup := newL2CacheForChatTests(t)
	defer cleanup()

	vector := []float64{0.11, 0.22, 0.33}
	cached := ChatCompletionResponse{
		ID:      "chatcmpl-cache-hit",
		Object:  "chat.completion",
		Created: 1710000000,
		Model:   "gpt-4",
		Choices: []Choice{{
			Index: 0,
			Message: ChatMessage{
				Role:    "assistant",
				Content: "from semantic cache",
			},
			FinishReason: "stop",
		}},
		Usage: Usage{
			PromptTokens:     3,
			CompletionTokens: 5,
			TotalTokens:      8,
		},
	}
	payload, err := json.Marshal(cached)
	if err != nil {
		t.Fatalf("marshal cached response failed: %v", err)
	}
	if err := l2Cache.Store(context.Background(), "seed-key", vector, string(payload), "gpt-4"); err != nil {
		t.Fatalf("store l2 cached response failed: %v", err)
	}

	workerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health", "/healthz":
			w.WriteHeader(http.StatusOK)
		case "/embeddings":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(embeddingworker.Response{
				Object: "list",
				Model:  "text-embedding-ada-002",
				Data: []embeddingworker.DataEntry{{
					Object:    "embedding",
					Embedding: vector,
					Index:     0,
				}},
				Usage: embeddingworker.Usage{TotalTokens: 3},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer workerServer.Close()

	workerClient := embeddingworker.NewClient(embeddingworker.Config{
		Address:      workerServer.URL,
		Timeout:      2 * time.Second,
		RetryMax:     1,
		RetryBackoff: time.Millisecond,
		HealthTTL:    time.Minute,
	})

	cfg := &config.Config{
		Cache: config.CacheConfig{Enabled: true},
	}
	router := gin.New()
	router.POST("/v1/chat/completions", ChatCompletion(cfg, nil, workerClient, nil, nil, l2Cache))

	reqBody := `{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d on l2 hit, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp ChatCompletionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Model != "gpt-4" {
		t.Fatalf("expected cached model gpt-4, got %q", resp.Model)
	}
	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content != "from semantic cache" {
		t.Fatalf("expected l2 cached content, got %+v", resp.Choices)
	}
}

func TestChatCompletion_WorkerUnhealthySkipsL2AndFallsThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	l2Cache, cleanup := newL2CacheForChatTests(t)
	defer cleanup()

	var embeddingCalls atomic.Int64
	workerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/embeddings" {
			embeddingCalls.Add(1)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer workerServer.Close()

	workerClient := embeddingworker.NewClient(embeddingworker.Config{
		Address:      workerServer.URL,
		Timeout:      2 * time.Second,
		RetryMax:     1,
		RetryBackoff: time.Millisecond,
		HealthTTL:    time.Minute,
	})

	cfg := &config.Config{
		Cache: config.CacheConfig{Enabled: true},
	}
	router := gin.New()
	router.POST("/v1/chat/completions", ChatCompletion(cfg, nil, workerClient, nil, nil, l2Cache))

	reqBody := `{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	}

	if got := embeddingCalls.Load(); got != 0 {
		t.Fatalf("expected no /embeddings calls when worker unhealthy, got %d", got)
	}

	var resp apierrors.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	if resp.Error.Code != apierrors.ErrCodeServiceUnavailable {
		t.Fatalf("expected code %q, got %q", apierrors.ErrCodeServiceUnavailable, resp.Error.Code)
	}
}

func TestMapUpstreamError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		status       int
		expectedHTTP int
		expectedCode apierrors.ErrorCode
	}{
		{
			name:         "unauthorized",
			status:       http.StatusUnauthorized,
			expectedHTTP: http.StatusUnauthorized,
			expectedCode: apierrors.ErrCodeInvalidAPIKey,
		},
		{
			name:         "bad gateway",
			status:       http.StatusBadGateway,
			expectedHTTP: http.StatusBadGateway,
			expectedCode: apierrors.ErrCodeBadGateway,
		},
		{
			name:         "service unavailable",
			status:       http.StatusServiceUnavailable,
			expectedHTTP: http.StatusServiceUnavailable,
			expectedCode: apierrors.ErrCodeModelOverloaded,
		},
		{
			name:         "rate limited",
			status:       http.StatusTooManyRequests,
			expectedHTTP: http.StatusTooManyRequests,
			expectedCode: apierrors.ErrCodeRateLimitExceeded,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := mapUpstreamError(&provider.UpstreamError{
				StatusCode: tc.status,
				Message:    "upstream error",
			})
			if err.Code != tc.expectedCode {
				t.Fatalf("expected code %q, got %q", tc.expectedCode, err.Code)
			}
			if got := err.HTTPStatus(); got != tc.expectedHTTP {
				t.Fatalf("expected http status %d, got %d", tc.expectedHTTP, got)
			}
		})
	}
}

func TestApplyWorkflowRoutePolicy(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Workflow: config.WorkflowConfig{
			RoutePolicies: map[string]string{
				"planning":      "gpt-3.5-turbo",
				"execution":     "gpt-4",
				"summarization": "gpt-3.5-turbo",
			},
		},
	}

	req := &ChatCompletionRequest{Model: "gpt-4"}
	applyWorkflowRoutePolicy(cfg, req, "planning")
	if req.Model != "gpt-3.5-turbo" {
		t.Fatalf("expected planning policy model %q, got %q", "gpt-3.5-turbo", req.Model)
	}

	applyWorkflowRoutePolicy(cfg, req, "unknown")
	if req.Model != "gpt-3.5-turbo" {
		t.Fatalf("expected unknown phase to keep model, got %q", req.Model)
	}
}

func newL2CacheForChatTests(t *testing.T) (*cache.L2Cache, func()) {
	t.Helper()

	redisServer, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	l2Cache := cache.NewL2Cache(redisClient, cache.L2CacheConfig{
		Enabled:             true,
		SimilarityThreshold: 0.95,
		TTL:                 3600,
		MaxSize:             100,
		KeyPrefix:           "cache:l2",
		VectorDim:           3,
	})

	cleanup := func() {
		_ = redisClient.Close()
		redisServer.Close()
	}
	return l2Cache, cleanup
}
