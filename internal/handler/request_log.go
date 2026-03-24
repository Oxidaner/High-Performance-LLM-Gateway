package handler

import (
	"context"
	"time"

	"llm-gateway/internal/middleware"
	"llm-gateway/internal/storage"
)

func persistRequestLog(
	pg *storage.PostgresClient,
	endpoint string,
	apiKeyID string,
	model string,
	statusCode int,
	latency time.Duration,
	cacheHit bool,
	totalTokens int,
) {
	if pg == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := pg.InsertRequestLog(ctx, storage.RequestLog{
		Endpoint:    endpoint,
		APIKeyID:    apiKeyID,
		Model:       model,
		StatusCode:  statusCode,
		LatencyMs:   int(latency.Milliseconds()),
		CacheHit:    cacheHit,
		TotalTokens: totalTokens,
	}); err != nil {
		middleware.Warn("failed to persist request log", middleware.Err(err))
	}
}
