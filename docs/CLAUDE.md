# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

High-Performance LLM Gateway - An enterprise-grade API gateway for managing LLM (Large Language Model) requests with multi-provider support, intelligent caching, and rate limiting.

## GitHub Repository

https://github.com/Oxidaner/High-Performance-LLM-Gateway

## Current Status (2026-02-16)

**Milestone**: M1 - Go HTTP 服务能运行 ✓

### Completed
- Go project initialization (go mod init)
- Gin framework setup
- HTTP server with health check
- Config loading (config.yaml)
- Zap logger integration
- Redis client (framework)
- PostgreSQL client (framework)
- API Key auth middleware (framework)
- Token bucket rate limiter (framework)
- L1 cache read/write (framework)
- Admin API: Key CRUD endpoints

### In Progress
- Learning Phase 0 (Python, FastAPI, Redis Vector)
- Connecting to actual Redis/PostgreSQL

## Architecture

### Components
- **Go Gateway** (:8080) - High-performance HTTP API gateway handling 10k+ QPS
- **Python Worker** (:8081) - Embedding generation service using sentence-transformers
- **Redis Stack** - L1 exact cache (SHA256 Hash) + L2 semantic cache (Vector similarity)
- **PostgreSQL** - Persistent storage for API keys, model configs, and request logs

### Key Design Decisions

1. **Layered Caching Strategy**
   - L1 (Exact Cache): Redis Hash with SHA256(prompt+model+temperature), <1ms latency
   - L2 (Semantic Cache): Redis Vector with similarity threshold >0.95, 10-50ms latency

2. **Independent Python Worker Deployment** (NOT sidecar)
   - Reason: Embedding is CPU-intensive, would compete with Go Gateway for resources

3. **Token Calculation in Go** (using tiktoken-go)
   - Avoids RPC calls to Python Worker for every request
   - Non-OpenAI models use character-count estimation

4. **Configuration Hot Reload**: K8s ConfigMap + fsnotify (no external config service)

## Common Commands

### Development
```bash
# Initialize Go module
go mod init llm-gateway

# Install dependencies
go mod tidy

# Run Go server
go run cmd/server/main.go

# Run Python worker
cd python-worker && pip install -r requirements.txt && uvicorn app.main:app --reload
```

### Testing
```bash
# Run Go tests
go test ./...

# Run specific test
go test ./internal/cache/... -v

# Run Python tests (if pytest configured)
pytest tests/
```

### Docker
```bash
# Build all services
docker-compose build

# Run development environment
docker-compose up -d

# Run with specific service
docker-compose up gateway
```

### Database
```bash
# Initialize PostgreSQL schema
psql -h localhost -U llm_gateway -d llm_gateway -f scripts/init_db.sql

# Run migrations (if using golang-migrate)
migrate -path migrations -database "postgres://..." up
```

### Deployment
```bash
# Deploy to Kubernetes
kubectl apply -f deployments/k8s/

# Check deployment status
kubectl get pods -l app=llm-gateway

# View logs
kubectl logs -f deployment/llm-gateway
```

## Code Structure

```
llm-gateway/                    # Go Gateway
├── cmd/server/main.go          # Entry point
├── internal/
│   ├── config/                 # Config loading with fsnotify
│   ├── handler/                # HTTP handlers (chat, embedding, admin)
│   ├── middleware/             # Auth, rate limit, logging
│   ├── service/
│   │   ├── cache/             # L1 + L2 cache logic
│   │   ├── provider/           # OpenAI, Claude, MiniMax adapters
│   │   ├── router.go          # Weighted round-robin routing
│   │   └── circuitbreaker.go  # Failure detection & fallback
│   ├── tokenizer/             # TikToken integration
│   └── storage/               # Redis & PostgreSQL clients

llm-worker/                     # Python Worker
├── app/main.py                 # FastAPI entry
├── app/routes/                 # /embeddings, /health
└── requirements.txt
```

## API Endpoints

### OpenAI-Compatible
- `POST /v1/chat/completions` - Chat completion
- `POST /v1/completions` - Text completion
- `POST /v1/embeddings` - Get embeddings
- `GET /v1/models` - List models

### Admin API
- `POST /api/v1/keys` - Create API key
- `GET /api/v1/keys` - List keys
- `DELETE /api/v1/keys/:id` - Delete key
- `GET /api/v1/stats` - Usage statistics

## Key Configuration Files

- `config.yaml` - Main configuration (models, rate limits, cache settings)
- `deployments/docker/docker-compose.yaml` - Local development
- `deployments/k8s/` - Kubernetes manifests

## Documentation

- **SPEC.md** - Full technical specification with architecture diagrams
- **Todo.md** - Development task tracking

## Performance Targets

- 10,000+ QPS throughput
- P99 latency < 500ms
- L1 cache hit: <1ms
- L2 cache hit: 10-50ms
- 80% cache hit rate (combined L1+L2)
