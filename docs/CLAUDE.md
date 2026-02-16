# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

High-Performance LLM Gateway - Enterprise-grade API gateway for LLM requests with multi-provider support, caching, and rate limiting.

## Current Status (2026-02-17)

**Milestone**: M1 - Go HTTP 服务框架完成 ✓

### 已完成
- Go 项目初始化 (go mod init)
- Gin 框架搭建
- HTTP 服务 + 健康检查
- Config 加载 (config.yaml)
- Zap 日志集成
- Redis 客户端 (框架)
- PostgreSQL 客户端 (框架)
- API Key 认证中间件 (框架)
- Token Bucket 限流 (框架)
- L1 缓存读写 (框架)
- Admin API: Key CRUD

### API Endpoints
- `GET /health` - 健康检查
- `POST /v1/chat/completions` - 聊天完成
- `POST /v1/embeddings` - 向量嵌入
- `GET /v1/models` - 模型列表
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
│   ├── config/             # 配置加载
│   ├── handler/            # HTTP 处理器
│   │   ├── chat.go
│   │   ├── embedding.go
│   │   └── admin.go
│   ├── logger/            # Zap 日志
│   ├── middleware/        # 认证、限流
│   │   ├── auth.go
│   │   └── ratelimit.go
│   └── storage/           # Redis、PostgreSQL
│       ├── redis.go
│       └── postgres.go
├── configs/config.yaml     # 配置文件
└── go.mod
```

## Commands

```bash
# 运行服务
go run cmd/server/main.go

# 编译
go build -o llm-gateway.exe ./cmd# 测试
go/server/main.go

 test ./...
```

## Important Notes

- Redis/PostgreSQL 连接失败时服务仍可运行（开发模式）
- 所有配置在 configs/config.yaml
- 日志使用 zap，支持 JSON/Console 格式
