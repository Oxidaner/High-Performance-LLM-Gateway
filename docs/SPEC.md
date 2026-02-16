# SPEC.md - 项目规格说明书

## 1. 项目概述

**项目名称**: High-Performance LLM Gateway
**类型**: 企业级 API 网关
**核心功能**: 管理 LLM 请求，支持多提供商、智能缓存、限流
**目标用户**: 需要统一管理 LLM API 的企业/开发者

---

## 2. 技术架构

### 2.1 组件

| 组件 | 端口 | 技术 |
|------|------|------|
| Go Gateway | 8080 | Go + Gin |
| Python Worker | 8081 | Python + FastAPI |
| Redis Stack | 6379 | 缓存 + 向量搜索 |
| PostgreSQL | 5432 | 持久化 |

### 2.2 请求流程

```
Client → Gateway → Auth → Rate Limit → Cache → LLM Provider
                  ↓
              PostgreSQL (API Key)
```

### 2.3 分层缓存

- **L1 精确缓存**: Redis Hash (SHA256 prompt), <1ms
- **L2 语义缓存**: Redis Vector (Embedding 相似度 >0.95), 10-50ms

---

## 3. API 接口

### 3.1 OpenAI 兼容接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /v1/chat/completions | 聊天完成 |
| POST | /v1/embeddings | 向量嵌入 |
| GET | /v1/models | 模型列表 |

### 3.2 Admin 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/keys | 创建 API Key |
| GET | /api/v1/keys | 列表 API Keys |
| DELETE | /api/v1/keys/:id | 删除 API Key |
| GET | /api/v1/stats | 使用统计 |

---

## 4. 功能规格

### 4.1 认证鉴权
- API Key 认证 (Bearer Token)
- Key 存储在 PostgreSQL
- Key 缓存到 Redis

### 4.2 限流
- Token Bucket 算法
- 全局限流 + 按模型限流
- 支持按 API Key 限流

### 4.3 缓存
- L1: SHA256 精确匹配
- L2: 向量相似度匹配

### 4.4 多提供商
- OpenAI (GPT-4, GPT-3.5)
- Anthropic (Claude)
- MiniMax
- 加权轮询 + 熔断降级

---

## 5. 性能目标

| 指标 | 目标 |
|------|------|
| QPS | 10,000+ |
| P99 延迟 | < 500ms |
| L1 缓存 | < 1ms |
| L2 缓存 | 10-50ms |
| 缓存命中率 | 80% |

---

## 6. 配置

配置文件: `configs/config.yaml`

主要配置项:
- server - 服务器配置
- logger - 日志配置
- database - PostgreSQL
- redis - Redis
- python_worker - Python 服务
- providers - LLM 提供商
- cache - 缓存配置
- ratelimit - 限流配置
- models - 模型配置

---

## 7. 依赖

### Go
- gin v1.9+
- go-redis v9+
- uber-go/zap
- gopkg.in/yaml.v3

### Python (Worker)
- fastapi
- uvicorn
- sentence-transformers
- redis

---

## 8. 部署

- Docker Compose 本地开发
- Kubernetes 生产部署
- Prometheus + Grafana 监控
