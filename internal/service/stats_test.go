package service

import (
	"net/http"
	"testing"
	"time"
)

func TestUsageStats_RecordAndSnapshot(t *testing.T) {
	t.Parallel()

	stats := NewUsageStats()
	stats.Record(RequestMetric{
		Endpoint:    "/v1/chat/completions",
		Model:       "gpt-4",
		StatusCode:  http.StatusOK,
		Latency:     120 * time.Millisecond,
		CacheHit:    true,
		TotalTokens: 42,
	})
	stats.Record(RequestMetric{
		Endpoint:    "/v1/chat/completions",
		Model:       "gpt-4",
		StatusCode:  http.StatusServiceUnavailable,
		Latency:     80 * time.Millisecond,
		CacheHit:    false,
		TotalTokens: 0,
	})

	snapshot := stats.Snapshot()
	if snapshot.TotalRequests != 2 {
		t.Fatalf("expected total requests 2, got %d", snapshot.TotalRequests)
	}
	if snapshot.TotalTokens != 42 {
		t.Fatalf("expected total tokens 42, got %d", snapshot.TotalTokens)
	}
	if snapshot.CacheHitRate != 0.5 {
		t.Fatalf("expected cache hit rate 0.5, got %f", snapshot.CacheHitRate)
	}

	model, ok := snapshot.Models["gpt-4"]
	if !ok {
		t.Fatalf("expected model stats for gpt-4")
	}
	if model.Requests != 2 {
		t.Fatalf("expected model requests 2, got %d", model.Requests)
	}
	if model.Errors != 1 {
		t.Fatalf("expected model errors 1, got %d", model.Errors)
	}
	if model.TotalTokens != 42 {
		t.Fatalf("expected model tokens 42, got %d", model.TotalTokens)
	}
}
