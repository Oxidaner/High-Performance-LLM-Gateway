package storage

import (
	"context"
	"database/sql"
	"fmt"

	"llm-gateway/internal/config"

	"github.com/lib/pq"
)

// PostgresClient wraps the PostgreSQL connection
// PostgresClient 包装 PostgreSQL 连接
type PostgresClient struct {
	db *sql.DB
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
