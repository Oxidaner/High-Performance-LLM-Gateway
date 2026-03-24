package embeddingworker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Config controls embedding worker client behavior.
type Config struct {
	Address      string
	Timeout      time.Duration
	RetryMax     int
	RetryBackoff time.Duration
	HealthTTL    time.Duration
}

// Client calls embedding worker with retry and health checks.
type Client struct {
	baseURL      string
	httpClient   *http.Client
	retryMax     int
	retryBackoff time.Duration
	healthTTL    time.Duration

	mu              sync.RWMutex
	lastHealthAt    time.Time
	lastHealthValue bool
}

// Request is embedding generation input.
type Request struct {
	Input interface{} `json:"input"`
	Model string      `json:"model"`
}

// Response is embedding generation output.
type Response struct {
	Object string      `json:"object"`
	Data   []DataEntry `json:"data"`
	Model  string      `json:"model"`
	Usage  Usage       `json:"usage"`
}

// DataEntry is one embedding vector entry.
type DataEntry struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// Usage is token usage from embedding worker.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// WorkerError wraps worker call failures.
type WorkerError struct {
	StatusCode int
	Message    string
	Err        error
}

func (e *WorkerError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "embedding worker error"
}

func (e *WorkerError) Retryable() bool {
	if e == nil {
		return false
	}
	if e.Err != nil {
		return true
	}
	switch e.StatusCode {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// NewClient builds embedding worker client with sane defaults.
func NewClient(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	retryMax := cfg.RetryMax
	if retryMax <= 0 {
		retryMax = 3
	}
	retryBackoff := cfg.RetryBackoff
	if retryBackoff <= 0 {
		retryBackoff = 300 * time.Millisecond
	}
	healthTTL := cfg.HealthTTL
	if healthTTL <= 0 {
		healthTTL = 10 * time.Second
	}

	return &Client{
		baseURL:      strings.TrimRight(strings.TrimSpace(cfg.Address), "/"),
		httpClient:   &http.Client{Timeout: timeout},
		retryMax:     retryMax,
		retryBackoff: retryBackoff,
		healthTTL:    healthTTL,
	}
}

// Healthy checks worker health with TTL cache.
func (c *Client) Healthy(ctx context.Context) bool {
	if c == nil || c.baseURL == "" {
		return false
	}

	now := time.Now()

	c.mu.RLock()
	lastAt := c.lastHealthAt
	lastValue := c.lastHealthValue
	c.mu.RUnlock()

	if !lastAt.IsZero() && now.Sub(lastAt) < c.healthTTL {
		return lastValue
	}

	healthy := c.probeHealth(ctx)
	c.mu.Lock()
	c.lastHealthAt = now
	c.lastHealthValue = healthy
	c.mu.Unlock()
	return healthy
}

// Generate calls embedding worker with retry.
func (c *Client) Generate(ctx context.Context, req Request) (*Response, *WorkerError) {
	if c == nil || c.baseURL == "" {
		return nil, &WorkerError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    "embedding worker is not configured",
		}
	}

	if strings.TrimSpace(req.Model) == "" {
		req.Model = "text-embedding-ada-002"
	}

	var lastErr *WorkerError
	backoff := c.retryBackoff
	for attempt := 0; attempt < c.retryMax; attempt++ {
		resp, err := c.generateOnce(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !err.Retryable() || attempt == c.retryMax-1 {
			break
		}

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, &WorkerError{
				StatusCode: http.StatusServiceUnavailable,
				Message:    "embedding worker request cancelled",
				Err:        ctx.Err(),
			}
		case <-timer.C:
		}
		backoff = backoff * 2
		if backoff > 2*time.Second {
			backoff = 2 * time.Second
		}
	}

	return nil, lastErr
}

func (c *Client) generateOnce(ctx context.Context, req Request) (*Response, *WorkerError) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, &WorkerError{
			StatusCode: http.StatusBadRequest,
			Message:    "failed to marshal embedding request",
			Err:        err,
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewBuffer(payload))
	if err != nil {
		return nil, &WorkerError{
			StatusCode: http.StatusBadGateway,
			Message:    "failed to build embedding worker request",
			Err:        err,
		}
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &WorkerError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    "failed to call embedding worker",
			Err:        err,
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &WorkerError{
			StatusCode: http.StatusBadGateway,
			Message:    "failed to read embedding worker response",
			Err:        err,
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &WorkerError{
			StatusCode: resp.StatusCode,
			Message:    parseWorkerErrorMessage(body),
		}
	}

	var out Response
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, &WorkerError{
			StatusCode: http.StatusBadGateway,
			Message:    "failed to parse embedding worker response",
			Err:        err,
		}
	}

	return &out, nil
}

func (c *Client) probeHealth(ctx context.Context) bool {
	for _, path := range []string{"/health", "/healthz"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
		if err != nil {
			continue
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		// Treat non-5xx as reachable to avoid hard dependency on specific health route.
		if resp.StatusCode < 500 {
			return true
		}
	}
	return false
}

func parseWorkerErrorMessage(body []byte) string {
	if len(body) == 0 {
		return "embedding worker returned empty error response"
	}

	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil {
		if envelope.Error.Message != "" {
			return envelope.Error.Message
		}
		if envelope.Message != "" {
			return envelope.Message
		}
	}

	msg := strings.TrimSpace(string(body))
	if len(msg) > 512 {
		msg = msg[:512]
	}
	return fmt.Sprintf("%s", msg)
}
