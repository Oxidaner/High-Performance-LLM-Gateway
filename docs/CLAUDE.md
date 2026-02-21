# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

High-Performance LLM Gateway - Enterprise-grade API gateway with AI Agent and RAG capabilities.

## Current Status (2026-02-21)

**Milestone**: M2 - OpenAI API 调用完成 ✓

### 已完成
- Go 项目初始化 (go mod init)
- Gin 框架搭建
- HTTP 服务 + 健康检查
- Config 加载 (config.yaml)
- Zap 日志集成
- Redis 客户端
- PostgreSQL 客户端
- API Key 认证中间件
- Token Bucket 限流
- L1 缓存读写
- Admin API: Key CRUD
- /v1/chat/completions 转发 OpenAI
- /v1/embeddings 转发 Python Worker

### 待实现 (Phase 6)
- AI Agent (ReAct/CoT 推理引擎)
- RAG (文档上传、向量检索)
- 智能重试 (指数退避)
- Prompt 优化 (缓存、压缩)
- 调用链观测 (OpenTelemetry/Jaeger)
- Python Worker 服务
- L2 语义缓存
- TikToken 精确计算
- 多模型负载均衡/熔断
- K8s 部署配置

### API Endpoints

#### LLM 网关
- `GET /health` - 健康检查
- `POST /v1/chat/completions` - 聊天完成
- `POST /v1/embeddings` - 向量嵌入
- `GET /v1/models` - 模型列表

#### RAG
- `POST /v1/rag/documents` - 上传文档
- `GET /v1/rag/documents` - 列表文档
- `POST /v1/rag/chat` - RAG 问答
- `GET /v1/rag/search` - 向量检索

#### Agent
- `POST /v1/agent/chat` - Agent 对话
- `GET /v1/agent/tools` - 工具列表

#### Admin
- `POST /api/v1/keys` - 创建 API Key
- `GET /api/v1/keys` - 列表 API Keys
- `DELETE /api/v1/keys/:id` - 删除 Key
- `GET /api/v1/stats` - 使用统计

### GitHub
https://github.com/Oxidaner/High-Performance-LLM-Gateway

## Architecture

```
llm-gateway/
├── cmd/server/main.go       # 入口
├── internal/
│   ├── agent/              # AI Agent 模块
│   │   ├── agent.go       # Agent 核心
│   │   ├── react.go       # ReAct 推理
│   │   ├── cot.go         # CoT 推理
│   │   ├── tools/         # 工具集
│   │   └── decision.go    # 自主决策
│   ├── rag/               # RAG 模块
│   │   ├── document.go    # 文档处理
│   │   ├── chunker.go    # 文本分块
│   │   ├── retriever.go  # 向量检索
│   │   └── knowledgebase.go
│   ├── config/            # 配置加载
│   ├── handler/           # HTTP 处理器
│   │   ├── chat.go
│   │   ├── embedding.go
│   │   ├── rag.go
│   │   ├── agent.go
│   │   └── admin.go
│   ├── middleware/        # 认证、限流
│   │   ├── auth.go
│   │   └── ratelimit.go
│   ├── service/          # 服务层
│   │   ├── router.go     # 负载均衡
│   │   ├── cache/        # L1/L2 缓存
│   │   └── provider/    # LLM 适配器
│   └── storage/          # Redis、PostgreSQL
│       ├── redis.go
│       └── postgres.go
├── configs/config.yaml    # 配置文件
└── go.mod
```

## Commands

```bash
# 运行服务
go run cmd/server/main.go

# 编译
go build -o llm-gateway.exe ./cmd/server

# 测试
go test ./...
```

## Core Features

| Feature | Status |
|---------|--------|
| LLM Gateway | ✓ |
| L1 Cache | ✓ |
| Token Rate Limit | ✓ |
| API Key Auth | ✓ |
| Admin API | ✓ |
| AI Agent | WIP |
| RAG | WIP |
| Smart Retry | WIP |
| Tracing | WIP |

## Important Notes

- Redis/PostgreSQL 连接失败时服务仍可运行（开发模式）
- 所有配置在 configs/config.yaml
- 日志使用 zap，支持 JSON/Console 格式
- Agent/RAG 为 Phase 6 主要开发目标
