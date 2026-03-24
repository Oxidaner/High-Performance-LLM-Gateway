package embeddingworker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientGenerate_RetryableFailureThenSuccess(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		current := calls.Add(1)
		if current < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"busy"}}`))
			return
		}

		_ = json.NewEncoder(w).Encode(Response{
			Object: "list",
			Model:  "text-embedding-ada-002",
			Data: []DataEntry{
				{
					Object:    "embedding",
					Embedding: []float64{0.1, 0.2, 0.3},
					Index:     0,
				},
			},
			Usage: Usage{
				TotalTokens: 3,
			},
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		Address:      server.URL,
		Timeout:      2 * time.Second,
		RetryMax:     3,
		RetryBackoff: 1 * time.Millisecond,
		HealthTTL:    1 * time.Second,
	})

	resp, err := client.Generate(context.Background(), Request{
		Model: "text-embedding-ada-002",
		Input: "hello",
	})
	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}
	if resp == nil || len(resp.Data) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("expected 3 calls (2 retries + 1 success), got %d", got)
	}
}

func TestClientGenerate_DoesNotRetryOnBadRequest(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		Address:      server.URL,
		Timeout:      2 * time.Second,
		RetryMax:     3,
		RetryBackoff: 1 * time.Millisecond,
		HealthTTL:    1 * time.Second,
	})

	_, err := client.Generate(context.Background(), Request{
		Model: "text-embedding-ada-002",
		Input: "hello",
	})
	if err == nil {
		t.Fatalf("expected bad request error")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected no retries for bad request, got calls=%d", got)
	}
}

func TestClientHealthy_UsesTTLCache(t *testing.T) {
	t.Parallel()

	var healthCalls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			healthCalls.Add(1)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(Config{
		Address:      server.URL,
		Timeout:      2 * time.Second,
		RetryMax:     1,
		RetryBackoff: 1 * time.Millisecond,
		HealthTTL:    1 * time.Minute,
	})

	if !client.Healthy(context.Background()) {
		t.Fatalf("expected healthy=true on first probe")
	}
	if !client.Healthy(context.Background()) {
		t.Fatalf("expected healthy=true from TTL cache")
	}
	if got := healthCalls.Load(); got != 1 {
		t.Fatalf("expected one health probe due TTL cache, got %d", got)
	}
}
