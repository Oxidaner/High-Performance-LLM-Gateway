# LLM Gateway Architecture

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
    end

    subgraph Redis["Redis Stack"]
        L1[L1 Exact Cache<br/>SHA256 Hash]
        L2[L2 Semantic Cache<br/>Vector Similarity]
    end

    subgraph Worker["Python Worker :8081"]
        Embed[Embedding<br/>sentence-transformers]
    end

    subgraph DB["PostgreSQL"]
        Keys[API Keys<br/>持久化]
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

    style Gateway fill:#e3f2fd,stroke:#1976d2
    style Redis fill:#e8f5e9,stroke:#388e3c
    style Worker fill:#fff3e0,stroke:#f57c00
    style LLM fill:#fce4ec,stroke:#c2185b
    style DB fill:#f5f5f5,stroke:#666666
```

## Request Flow

1. **Client** sends request to **Go Gateway** (:8080)
2. **API Key Auth** validates the API key (checks PostgreSQL cache)
3. **Rate Limit** enforces token bucket rate limiting
4. **Cache Router** checks:
   - **L1 Exact Cache**: SHA256(prompt+model+temperature) - <1ms
   - **L2 Semantic Cache**: Vector similarity >0.95 - 10-50ms
5. **LLM Router** forwards to provider with weighted round-robin
6. **Response** returned to client

## Component Ports

| Component | Port | Description |
|-----------|------|-------------|
| Go Gateway | 8080 | Main API server |
| Python Worker | 8081 | Embedding service |
| Redis Stack | 6379 | Cache + Vector search |
| PostgreSQL | 5432 | Persistent storage |
