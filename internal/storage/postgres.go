package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"llm-gateway/internal/config"

	"github.com/lib/pq"
)

// PostgresClient wraps the PostgreSQL connection
// PostgresClient 包装 PostgreSQL 连接
type PostgresClient struct {
	db *sql.DB
}

// RequestLog is one persisted gateway request log entry.
type RequestLog struct {
	Endpoint    string
	APIKeyID    string
	Model       string
	StatusCode  int
	LatencyMs   int
	CacheHit    bool
	TotalTokens int
}

// UsageModelStats contains per-model aggregated stats from persisted logs.
type UsageModelStats struct {
	Requests      int64   `json:"requests"`
	Errors        int64   `json:"errors"`
	TotalTokens   int64   `json:"total_tokens"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	LastStatus    int     `json:"last_status"`
	LastUpdatedAt string  `json:"last_updated_at"`
}

// UsageSnapshot is a persisted usage summary.
type UsageSnapshot struct {
	TotalRequests int64                      `json:"total_requests"`
	TotalTokens   int64                      `json:"total_tokens"`
	TotalCost     float64                    `json:"total_cost"`
	CacheHitRate  float64                    `json:"cache_hit_rate"`
	Models        map[string]UsageModelStats `json:"models"`
}

// NewPostgres creates a new PostgreSQL client
// NewPostgres 创建新的 PostgreSQL 客户端
func NewPostgres(cfg config.DatabaseConfig) (*PostgresClient, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return &PostgresClient{db: db}, nil
}

// Close closes the database connection
func (p *PostgresClient) Close() error {
	return p.db.Close()
}

// DB returns the underlying database connection
func (p *PostgresClient) DB() *sql.DB {
	return p.db
}

// APIKey represents an API key record
type APIKey struct {
	ID        string
	KeyHash   string
	Name      string
	RateLimit int
	IsActive  bool
	ExpiresAt pq.NullTime
	CreatedAt pq.NullTime
	UpdatedAt pq.NullTime
}

// GetAPIKeyByHash retrieves an API key by its hash
// GetAPIKeyByHash 根据哈希值检索 API 密钥
func (p *PostgresClient) GetAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	query := `
		SELECT id, key_hash, name, rate_limit, is_active, expires_at, created_at, updated_at
		FROM api_keys
		WHERE key_hash = $1 AND is_active = true
	`

	var key APIKey
	err := p.db.QueryRowContext(ctx, query, keyHash).Scan(
		&key.ID, &key.KeyHash, &key.Name, &key.RateLimit,
		&key.IsActive, &key.ExpiresAt, &key.CreatedAt, &key.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &key, nil
}

// CreateAPIKey creates a new API key
func (p *PostgresClient) CreateAPIKey(ctx context.Context, keyHash, name string, rateLimit int) (*APIKey, error) {
	query := `
		INSERT INTO api_keys (key_hash, name, rate_limit, is_active)
		VALUES ($1, $2, $3, true)
		RETURNING id, key_hash, name, rate_limit, is_active, expires_at, created_at, updated_at
	`

	var key APIKey
	err := p.db.QueryRowContext(ctx, query, keyHash, name, rateLimit).Scan(
		&key.ID, &key.KeyHash, &key.Name, &key.RateLimit,
		&key.IsActive, &key.ExpiresAt, &key.CreatedAt, &key.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &key, nil
}

// ListAPIKeys lists all API keys
func (p *PostgresClient) ListAPIKeys(ctx context.Context) ([]APIKey, error) {
	query := `
		SELECT id, key_hash, name, rate_limit, is_active, expires_at, created_at, updated_at
		FROM api_keys
		ORDER BY created_at DESC
	`

	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var key APIKey
		if err := rows.Scan(
			&key.ID, &key.KeyHash, &key.Name, &key.RateLimit,
			&key.IsActive, &key.ExpiresAt, &key.CreatedAt, &key.UpdatedAt,
		); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// DeleteAPIKey deletes an API key
func (p *PostgresClient) DeleteAPIKey(ctx context.Context, id string) error {
	query := `DELETE FROM api_keys WHERE id = $1`
	_, err := p.db.ExecContext(ctx, query, id)
	return err
}

// EnsureRequestLogSchema creates request log table and indexes if they do not exist.
func (p *PostgresClient) EnsureRequestLogSchema(ctx context.Context) error {
	if p == nil || p.db == nil {
		return fmt.Errorf("postgres client is not initialized")
	}

	ddl := []string{
		`CREATE TABLE IF NOT EXISTS request_logs (
			id BIGSERIAL PRIMARY KEY,
			endpoint VARCHAR(128) NOT NULL,
			api_key_id TEXT,
			model VARCHAR(128) NOT NULL,
			status_code INT NOT NULL,
			latency_ms INT NOT NULL,
			cache_hit BOOLEAN NOT NULL DEFAULT FALSE,
			total_tokens INT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_created_at ON request_logs(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_request_logs_model_created_at ON request_logs(model, created_at DESC)`,
	}

	for _, stmt := range ddl {
		if _, err := p.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// InsertRequestLog inserts a request log record.
func (p *PostgresClient) InsertRequestLog(ctx context.Context, entry RequestLog) error {
	if p == nil || p.db == nil {
		return fmt.Errorf("postgres client is not initialized")
	}

	if entry.Endpoint == "" {
		entry.Endpoint = "unknown"
	}
	if entry.Model == "" {
		entry.Model = "unknown"
	}
	if entry.LatencyMs < 0 {
		entry.LatencyMs = 0
	}
	if entry.TotalTokens < 0 {
		entry.TotalTokens = 0
	}

	query := `
		INSERT INTO request_logs (endpoint, api_key_id, model, status_code, latency_ms, cache_hit, total_tokens)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := p.db.ExecContext(
		ctx,
		query,
		entry.Endpoint,
		entry.APIKeyID,
		entry.Model,
		entry.StatusCode,
		entry.LatencyMs,
		entry.CacheHit,
		entry.TotalTokens,
	)
	return err
}

// GetUsageSnapshot returns aggregate usage stats from persisted logs.
func (p *PostgresClient) GetUsageSnapshot(ctx context.Context) (*UsageSnapshot, error) {
	if p == nil || p.db == nil {
		return nil, fmt.Errorf("postgres client is not initialized")
	}

	snapshot := &UsageSnapshot{
		Models: map[string]UsageModelStats{},
	}

	totalQuery := `
		SELECT
			COUNT(*) AS total_requests,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			COALESCE(AVG(CASE WHEN cache_hit THEN 1.0 ELSE 0.0 END), 0) AS cache_hit_rate
		FROM request_logs
	`
	if err := p.db.QueryRowContext(ctx, totalQuery).Scan(
		&snapshot.TotalRequests,
		&snapshot.TotalTokens,
		&snapshot.CacheHitRate,
	); err != nil {
		return nil, err
	}

	modelQuery := `
		WITH aggregates AS (
			SELECT
				model,
				COUNT(*) AS requests,
				SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) AS errors,
				COALESCE(SUM(total_tokens), 0) AS total_tokens,
				COALESCE(AVG(latency_ms), 0) AS avg_latency_ms
			FROM request_logs
			GROUP BY model
		),
		latest AS (
			SELECT DISTINCT ON (model)
				model,
				status_code,
				created_at
			FROM request_logs
			ORDER BY model, created_at DESC
		)
		SELECT
			a.model,
			a.requests,
			a.errors,
			a.total_tokens,
			a.avg_latency_ms,
			COALESCE(l.status_code, 0) AS last_status,
			COALESCE(l.created_at, NOW()) AS last_updated_at
		FROM aggregates a
		LEFT JOIN latest l ON l.model = a.model
	`

	rows, err := p.db.QueryContext(ctx, modelQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			model         string
			requests      int64
			errors        int64
			totalTokens   int64
			avgLatencyMs  float64
			lastStatus    int
			lastUpdatedAt time.Time
		)

		if err := rows.Scan(
			&model,
			&requests,
			&errors,
			&totalTokens,
			&avgLatencyMs,
			&lastStatus,
			&lastUpdatedAt,
		); err != nil {
			return nil, err
		}

		snapshot.Models[model] = UsageModelStats{
			Requests:      requests,
			Errors:        errors,
			TotalTokens:   totalTokens,
			AvgLatencyMs:  avgLatencyMs,
			LastStatus:    lastStatus,
			LastUpdatedAt: lastUpdatedAt.UTC().Format(time.RFC3339),
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Pricing is not wired yet.
	snapshot.TotalCost = 0

	return snapshot, nil
}
