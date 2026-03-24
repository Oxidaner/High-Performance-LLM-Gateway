package provider

import (
	"context"
	"net/http"
	"testing"
	"time"

	"llm-gateway/internal/service"
)

type stubClient struct {
	name   string
	calls  int
	fn     func(req ChatCompletionRequest) (*ChatCompletionResponse, *UpstreamError)
	models []string
}

func (s *stubClient) Name() string {
	return s.name
}

func (s *stubClient) ChatCompletion(_ context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, *UpstreamError) {
	s.calls++
	s.models = append(s.models, req.Model)
	if s.fn == nil {
		return nil, &UpstreamError{StatusCode: http.StatusServiceUnavailable, Message: "not implemented"}
	}
	return s.fn(req)
}

func TestRegistry_FallbackToNextModel(t *testing.T) {
	t.Parallel()

	openai := &stubClient{
		name: "openai",
		fn: func(req ChatCompletionRequest) (*ChatCompletionResponse, *UpstreamError) {
			return nil, &UpstreamError{
				StatusCode: http.StatusServiceUnavailable,
				Message:    "openai overloaded",
			}
		},
	}
	anthropic := &stubClient{
		name: "anthropic",
		fn: func(req ChatCompletionRequest) (*ChatCompletionResponse, *UpstreamError) {
			return &ChatCompletionResponse{
				ID:      "1",
				Object:  "chat.completion",
				Created: 1,
				Model:   req.Model,
			}, nil
		},
	}

	r := &Registry{
		providers: map[string]Client{
			"openai":    openai,
			"anthropic": anthropic,
		},
		modelRoutes: map[string][]routeCandidate{
			"gpt-4": {
				{
					modelName:    "gpt-4",
					providerName: "openai",
					weight:       1,
				},
			},
			"claude-3-haiku": {
				{
					modelName:    "claude-3-haiku",
					providerName: "anthropic",
					weight:       1,
				},
			},
		},
		modelFallbacks: map[string]string{
			"gpt-4": "claude-3-haiku",
		},
		circuits: map[string]*service.CircuitBreaker{
			"openai:gpt-4":             service.NewCircuitBreaker(3, 30*time.Second),
			"anthropic:claude-3-haiku": service.NewCircuitBreaker(3, 30*time.Second),
		},
		nowFunc:  func() time.Time { return time.Unix(100, 0) },
		randIntn: func(n int) int { return 0 },
	}

	resp, err := r.ChatCompletion(context.Background(), ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []ChatMessage{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("expected success via fallback, got err: %+v", err)
	}
	if resp == nil || resp.Model != "claude-3-haiku" {
		t.Fatalf("expected fallback model response, got %+v", resp)
	}
	if openai.calls != 1 {
		t.Fatalf("expected openai calls=1, got %d", openai.calls)
	}
	if anthropic.calls != 1 {
		t.Fatalf("expected anthropic calls=1, got %d", anthropic.calls)
	}
}

func TestRegistry_CircuitBreakerSkipsOpenRoute(t *testing.T) {
	t.Parallel()

	openai := &stubClient{
		name: "openai",
		fn: func(req ChatCompletionRequest) (*ChatCompletionResponse, *UpstreamError) {
			return nil, &UpstreamError{
				StatusCode: http.StatusServiceUnavailable,
				Message:    "openai overloaded",
			}
		},
	}
	anthropic := &stubClient{
		name: "anthropic",
		fn: func(req ChatCompletionRequest) (*ChatCompletionResponse, *UpstreamError) {
			return &ChatCompletionResponse{
				ID:      "1",
				Object:  "chat.completion",
				Created: 1,
				Model:   req.Model,
			}, nil
		},
	}

	now := time.Unix(100, 0)
	breaker := service.NewCircuitBreaker(1, 30*time.Second)
	r := &Registry{
		providers: map[string]Client{
			"openai":    openai,
			"anthropic": anthropic,
		},
		modelRoutes: map[string][]routeCandidate{
			"gpt-4": {
				{modelName: "gpt-4", providerName: "openai", weight: 1},
			},
			"claude-3-haiku": {
				{modelName: "claude-3-haiku", providerName: "anthropic", weight: 1},
			},
		},
		modelFallbacks: map[string]string{
			"gpt-4": "claude-3-haiku",
		},
		circuits: map[string]*service.CircuitBreaker{
			"openai:gpt-4":             breaker,
			"anthropic:claude-3-haiku": service.NewCircuitBreaker(1, 30*time.Second),
		},
		nowFunc: func() time.Time { return now },
		randIntn: func(n int) int {
			return 0
		},
	}

	_, err1 := r.ChatCompletion(context.Background(), ChatCompletionRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err1 != nil {
		t.Fatalf("expected first request success via fallback, got err: %+v", err1)
	}
	if openai.calls != 1 {
		t.Fatalf("expected openai calls=1 after first request, got %d", openai.calls)
	}

	_, err2 := r.ChatCompletion(context.Background(), ChatCompletionRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err2 != nil {
		t.Fatalf("expected second request success via fallback, got err: %+v", err2)
	}
	if openai.calls != 1 {
		t.Fatalf("expected openai calls to remain 1 due open circuit, got %d", openai.calls)
	}
	if anthropic.calls != 2 {
		t.Fatalf("expected anthropic calls=2, got %d", anthropic.calls)
	}
}

func TestRegistry_WeightedOrderPrefersHigherWeight(t *testing.T) {
	t.Parallel()

	r := &Registry{
		randIntn: func(n int) int {
			// Pick the last slot in weighted range, which should favor higher weight.
			return n - 1
		},
	}

	candidates := []routeCandidate{
		{modelName: "gpt-4", providerName: "openai", weight: 1},
		{modelName: "gpt-4", providerName: "anthropic", weight: 3},
	}

	ordered := r.weightedOrder(candidates)
	if len(ordered) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(ordered))
	}
	if ordered[0].providerName != "anthropic" {
		t.Fatalf("expected high weight candidate first, got %s", ordered[0].providerName)
	}
}
