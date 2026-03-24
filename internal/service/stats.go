package service

import (
	"sync"
	"time"
)

// RequestMetric is one request-level usage event.
type RequestMetric struct {
	Endpoint    string
	Model       string
	StatusCode  int
	Latency     time.Duration
	CacheHit    bool
	TotalTokens int
}

// ModelUsage aggregates per-model metrics.
type ModelUsage struct {
	Requests      int64   `json:"requests"`
	Errors        int64   `json:"errors"`
	TotalTokens   int64   `json:"total_tokens"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	LastStatus    int     `json:"last_status"`
	LastUpdatedAt string  `json:"last_updated_at"`
}

// UsageSnapshot is an immutable copy of current usage stats.
type UsageSnapshot struct {
	TotalRequests int64                 `json:"total_requests"`
	TotalTokens   int64                 `json:"total_tokens"`
	TotalCost     float64               `json:"total_cost"`
	CacheHitRate  float64               `json:"cache_hit_rate"`
	Models        map[string]ModelUsage `json:"models"`
}

// UsageStats is an in-memory real-time usage aggregator.
type UsageStats struct {
	mu sync.RWMutex

	totalRequests int64
	totalTokens   int64
	cacheHits     int64
	models        map[string]*ModelUsage
}

var defaultUsageStats = NewUsageStats()

// NewUsageStats creates a new usage aggregator.
func NewUsageStats() *UsageStats {
	return &UsageStats{
		models: make(map[string]*ModelUsage),
	}
}

// DefaultUsageStats returns the singleton stats collector.
func DefaultUsageStats() *UsageStats {
	return defaultUsageStats
}

// Record records one request event.
func (s *UsageStats) Record(metric RequestMetric) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.totalRequests++
	if metric.TotalTokens > 0 {
		s.totalTokens += int64(metric.TotalTokens)
	}
	if metric.CacheHit {
		s.cacheHits++
	}

	model := metric.Model
	if model == "" {
		model = "unknown"
	}

	entry, exists := s.models[model]
	if !exists {
		entry = &ModelUsage{}
		s.models[model] = entry
	}

	entry.Requests++
	if metric.StatusCode >= 400 {
		entry.Errors++
	}
	if metric.TotalTokens > 0 {
		entry.TotalTokens += int64(metric.TotalTokens)
	}

	latencyMs := float64(metric.Latency.Milliseconds())
	if entry.Requests == 1 {
		entry.AvgLatencyMs = latencyMs
	} else {
		entry.AvgLatencyMs = ((entry.AvgLatencyMs * float64(entry.Requests-1)) + latencyMs) / float64(entry.Requests)
	}
	entry.LastStatus = metric.StatusCode
	entry.LastUpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

// Snapshot returns a copy of current stats.
func (s *UsageStats) Snapshot() UsageSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	models := make(map[string]ModelUsage, len(s.models))
	for model, usage := range s.models {
		models[model] = *usage
	}

	cacheHitRate := 0.0
	if s.totalRequests > 0 {
		cacheHitRate = float64(s.cacheHits) / float64(s.totalRequests)
	}

	return UsageSnapshot{
		TotalRequests: s.totalRequests,
		TotalTokens:   s.totalTokens,
		TotalCost:     0, // pricing metadata is not wired yet
		CacheHitRate:  cacheHitRate,
		Models:        models,
	}
}
