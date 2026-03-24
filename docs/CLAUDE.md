# CLAUDE.md

This file gives coding agents a realistic overview of the repository.

## Project Summary

`High-Performance-LLM-Gateway` is an OpenAI-compatible LLM gateway in Go.

Treat this project as:

- a gateway and request-governance system first
- an agent-infrastructure support layer second

Do not treat this project as a finished agent or RAG platform.

## What Exists

- Gin-based HTTP server and config loading
- API key auth middleware
- global + model-aware rate limiting
- Redis and PostgreSQL storage wiring
- chat completion and embedding forwarding
- model listing and admin key CRUD
- L1 exact cache in chat path
- L2 semantic cache in live chat path (`L1 -> L2 -> provider`)
- provider abstraction, registry, weighted routing, fallback, circuit breaker
- embedding worker client with health checks and retry policy
- request logging and PostgreSQL-backed request records
- stats endpoint with DB aggregate fallback behavior
- workflow lineage tracing and replay JSONL output
- workflow summary endpoints
- OpenTelemetry spans and OTLP export support
- Docker local integration stack and load test scripts

## What Is Still Missing

- richer pricing model for workflow cost accounting
- durable workflow summary pipeline (current aggregation is in-memory)
- full agent runtime and full RAG product surface (explicitly de-prioritized)

## Current API Surface

### Public

- `GET /health`
- `POST /v1/chat/completions`
- `POST /v1/embeddings`
- `GET /v1/models`

### Admin

- `POST /api/v1/keys`
- `GET /api/v1/keys`
- `DELETE /api/v1/keys/:id`
- `GET /api/v1/stats`
- `GET /api/v1/workflows/:session_id/summary`
- `GET /api/v1/workflows/summaries`

## Development Direction

When extending this repository, prefer this order:

1. improve correctness and reliability of existing gateway paths
2. improve routing/governance/observability depth
3. add narrowly scoped agent-facing infrastructure hooks

Avoid broad platform expansion unless explicitly requested:

- full agent runtime
- broad RAG subsystem
- large multi-tool orchestration layer

## Directory Notes

```text
cmd/server/main.go          server bootstrap and route registration
internal/handler/           request handlers
internal/middleware/        auth, logging, rate limiting, tracing middleware
internal/service/cache/     L1 and L2 cache implementations
internal/service/provider/  provider adapters and registry
internal/service/workflow/  workflow trace and summary services
internal/service/embeddingworker/ embedding worker client
internal/storage/           Redis and PostgreSQL clients
```
