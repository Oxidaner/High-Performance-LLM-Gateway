# Development Roadmap

Last updated: 2026-03-24

## Principle

Build this as an `OpenAI-compatible inference gateway` first.

Add agent-related capabilities only when they strengthen gateway behavior:

- workflow-aware routing
- request lineage and replay
- cost and latency observability for multi-step workloads

## Current Reality

Implemented and in active path:

- provider abstraction + registry
- weighted routing, fallback, and circuit breaker
- API key auth + global/model-aware rate limiting
- L1 exact cache and L2 semantic cache in live chat path
- embedding worker health check + retry strategy
- request logging + durable PostgreSQL request logs (with fallback)
- stats endpoint backed by PostgreSQL aggregates when available
- workflow trace model, replay JSONL log, and workflow summary APIs
- phase-aware route policy (`planning` / `execution` / `summarization`)
- OpenTelemetry spans and OTLP export support
- docker local integration stack + load test scripts
- unit tests for core gateway behavior

Not production-complete yet:

- richer pricing model for accurate workflow cost accounting
- durable workflow summary pipeline (current summary aggregation is in-memory)

## Priority Status

### P0: Make The Existing Gateway Correct

- [x] add unit tests for auth, rate limiting, cache, and request validation
- [x] fix rate limit model extraction for JSON requests
- [x] separate provider logic from `internal/handler/chat.go`
- [x] standardize upstream error mapping
- [x] document the current supported model list and request constraints

### P1: Make The Gateway Operationally Useful

- [x] implement provider interface and provider registry
- [x] add weighted routing and fallback
- [x] add circuit breaker
- [x] add request logging with model, latency, status, and cache hit
- [x] make `/api/v1/stats` read from real usage data

### P2: Add Agent Infrastructure Hooks

- [x] design a trace model for one multi-step workflow
- [x] record request lineage: session, step, tool, upstream model, latency
- [x] add replay-friendly log format
- [x] add route policies for planning vs execution vs summarization
- [x] expose workflow cost and latency summaries

### P3: Optional Expansion

- [x] wire L2 semantic cache into the live request path
- [x] add embedding worker health and retry strategy
- [x] add OpenTelemetry spans
- [x] add Docker-based local integration environment
- [x] add basic load test scripts

### P4: Next Work

- [ ] workflow pricing model refinement
- [ ] durable workflow summary pipeline

## Explicit De-Prioritization

Not the next thing to build:

- a general-purpose agent framework
- a full RAG product surface
- a large tool ecosystem
- a broad orchestration layer

These can be revisited after gateway stability and observability are strong enough.
