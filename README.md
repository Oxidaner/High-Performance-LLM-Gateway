# English | [中文版](./README-zh.md)

# High-Performance LLM Gateway

Enterprise-grade API gateway for LLM requests with multi-provider support, intelligent caching, AI Agent, and RAG capabilities.

## Architecture

```mermaid
flowchart TB
    subgraph Client["Client"]
        Req[User Request]
    end

    subgraph Gateway["Go Gateway :8080"]
        Auth[API Key Auth]
        Rate[Rate Limit]
        Cache[Cache Router]
        Router[LLM Router]
        Agent[AI Agent<br/>ReAct/CoT]
        RAG[RAG Engine<br/>Vector Search]
    end

    subgraph Redis["Redis Stack"]
        L1[L1 Exact Cache<br/>SHA256 Hash]
        L2[L2 Semantic Cache<br/>Vector Similarity]
        Vector[Vector Index<br/>RAG Documents]
    end

    subgraph Worker["Python Worker :8081"]
        Embed[Embedding<br/>sentence-transformers]
    end

    subgraph DB["PostgreSQL"]
        Keys[API Keys<br/>Persistence]
        Docs[RAG Documents<br/>Knowledge Base]
    end

    subgraph LLM["LLM Providers"]
        OpenAI[OpenAI<br/>GPT-4/3.5]
        Claude[Anthropic<br/>Claude]
        MiniMax[MiniMax]
    end

    Req --> Auth
    Auth -.->|Verify Key| Keys
    Auth --> Rate
    Rate --> Cache
    Cache -->|L1 Hit| L1
    L1 -->|Return| Req
    Cache -->|L1 Miss| L2
    L2 -->|L2 Hit| Embed
    Embed -->|Return Vector| L2
    L2 -->|L2 Miss| Router
    Router --> OpenAI
    Router --> Claude
    Router --> MiniMax

    Agent --> RAG
    Agent --> Embed
    RAG --> Vector

    style Gateway fill:#e3f2fd,stroke:#1976d2
    style Redis fill:#e8f5e9,stroke:#388e3c
    style Worker fill:#fff3e0,stroke:#f57c00
    style LLM fill:#fce4ec,stroke:#c2185b
    style DB fill:#f5f5f5,stroke:#666666
```

## Features

- **Multi-Provider Support**: OpenAI, Anthropic (Claude), MiniMax
- **Layered Caching**:
  - L1 Exact Cache: Redis Hash (SHA256), <1ms latency
  - L2 Semantic Cache: Redis Vector (Embedding similarity >0.95), 10-50ms latency
- **Token Rate Limiting**: Token bucket algorithm with TikToken Go
- **High Performance**: 10,000+ QPS throughput
- **AI Agent**:
  - ReAct/CoT reasoning engine
  - Tool calling (web search, database query, API call)
  - Autonomous decision making
- **RAG**:
  - Document upload and processing
  - Vector storage with Redis
  - Knowledge base management
- **Intelligent Retry**: Exponential backoff with retryable error detection
- **Prompt Optimization**: System prompt caching, history compression
- **Distributed Tracing**: OpenTelemetry / Jaeger integration
- **Authentication**: API Key based auth with Redis caching
- **Admin API**: Key management and usage statistics

## Quick Start

### Prerequisites

- Go 1.21+
- Redis (for caching)
- PostgreSQL (for persistence)

### Run Locally

```bash
# Clone the repository
git clone https://github.com/Oxidaner/High-Performance-LLM-Gateway.git
cd High-Performance-LLM-Gateway

# Copy config and set your API keys
cp configs/config.yaml configs/config.yaml
# Edit config.yaml with your API keys

# Run the server
go run cmd/server/main.go
```

### Configuration

Edit `configs/config.yaml`:

```yaml
server:
  host: 0.0.0.0
  port: 8080
  mode: debug

logger:
  level: info
  format: console
  output_path: stdout

providers:
  openai:
    api_key: your-openai-key
    base_url: https://api.openai.com/v1
  anthropic:
    api_key: your-anthropic-key
```

## API Endpoints

### OpenAI-Compatible API

```bash
# Chat completions
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# List models
curl http://localhost:8080/v1/models

# Get embeddings
curl -X POST http://localhost:8080/v1/embeddings \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "text-embedding-ada-002",
    "input": "Hello world"
  }'
```

### RAG API

```bash
# Upload document
curl -X POST http://localhost:8080/v1/rag/documents \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -F "file=@document.txt"

# RAG chat
curl -X POST http://localhost:8080/v1/rag/chat \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "question": "What is the main topic of the documents?"
  }'
```

### Agent API

```bash
# Agent chat (with reasoning)
curl -X POST http://localhost:8080/v1/agent/chat \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "What was the total cost of GPT-4 last week?"
  }'

# List available tools
curl http://localhost:8080/v1/agent/tools
```

### Admin API

```bash
# Create API key
curl -X POST http://localhost:8080/api/v1/keys \
  -H "Content-Type: application/json" \
  -d '{"name": "my-key", "rate_limit": 1000}'

# List API keys
curl http://localhost:8080/api/v1/keys

# Get usage stats
curl http://localhost:8080/api/v1/stats
```

## Performance Targets

| Metric | Target |
|--------|--------|
| QPS | 10,000+ |
| P99 Latency | < 500ms |
| L1 Cache Hit | < 1ms |
| L2 Cache Hit | 10-50ms |
| Cache Hit Rate | 80% |
| LLM Success Rate | > 99.5% |

## Project Structure

```
llm-gateway/
├── cmd/server/           # Entry point
├── internal/
│   ├── agent/          # AI Agent module
│   │   ├── agent.go   # Agent core
│   │   ├── react.go    # ReAct reasoning
│   │   ├── cot.go      # CoT reasoning
│   │   └── tools/      # Tool implementations
│   ├── rag/            # RAG module
│   │   ├── document.go # Document processing
│   │   ├── chunker.go  # Text chunking
│   │   └── retriever.go # Vector retrieval
│   ├── config/         # Configuration loading
│   ├── handler/        # HTTP handlers
│   ├── middleware/     # Auth, rate limiting
│   ├── service/        # Router, cache, providers
│   └── storage/       # Redis, PostgreSQL clients
├── configs/             # Configuration files
├── docs/                # Documentation
└── go.mod
```

## Tech Stack

- **Gateway**: Go + Gin
- **AI Worker**: Python + FastAPI + sentence-transformers
- **Cache**: Redis Stack (vector search + caching)
- **Database**: PostgreSQL
- **Tracing**: OpenTelemetry + Jaeger
- **Deployment**: Kubernetes

## License

MIT
