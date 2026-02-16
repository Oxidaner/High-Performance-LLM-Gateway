# High-Performance LLM Gateway 开发计划

> 基于张世斌简历评估 | 项目周期: 10-14 周

---

## 一、学习阶段 (Phase 0) - Week 1-4

> ⚡ **快速入门网址合集**
> - Python: https://www.runoob.com/python3/python3-tutorial.html (菜鸟教程, 最快上手)
> - FastAPI: https://fastapi.tiangolo.com/zh/tutorial/
> - Redis Vector: https://redis.io/docs/stack/search/vector-similarity/

### Task 0.1: Python 基础入门 (Day 1-7)

> 推荐: https://www.runoob.com/python3/python3-tutorial.html (1-2天速成)
> 或: https://docs.python.org/3/tutorial/ (官方文档)

- [x] Task 0.1.1: 安装 Python 3.11+ 环境
- [ ] Task 0.1.2: 掌握变量、数据类型、运算符
- [ ] Task 0.1.3: 掌握条件语句、循环语句
- [ ] Task 0.1.4: 掌握函数定义与调用
- [ ] Task 0.1.5: 掌握列表，元组、字典操作
- [ ] Task 0.1.6: 掌握模块导入与使用 (import)
- [ ] Task 0.1.7: 掌握 pip 包管理器的使用

### Task 0.2: FastAPI 入门 (Day 8-14)

> 推荐: https://fastapi.tiangolo.com/zh/tutorial/first-steps/ (官方教程)
> 实战: 跟着写一个 Todo API

- [ ] Task 0.2.1: 安装 FastAPI 和 uvicorn
- [ ] Task 0.2.2: 编写第一个 FastAPI 应用 (Hello World)
- [ ] Task 0.2.3: 掌握路由定义 (@app.get/post)
- [ ] Task 0.2.4: 掌握 Query 参数和 Path 参数
- [ ] Task 0.2.5: 掌握 Request Body (Pydantic 模型)
- [ ] Task 0.2.6: 编写一个简单的 Todo API (增删改查)
- [ ] Task 0.2.7: 掌握 Python 虚拟环境 venv 的使用

### Task 0.3: Redis Vector 入门 (Day 15-21)

> 推荐: https://redis.io/docs/stack/search/vector-similarity/ (向量搜索)
> 快速上手: https://redis.io/docs/stack/search/quick-start/ (5分钟入门)

- [ ] Task 0.3.1: 安装 Docker Desktop (如果没有)
- [ ] Task 0.3.2: 使用 Docker 运行 Redis Stack 容器
- [ ] Task 0.3.3: 使用 redis-cli 连接 Redis
- [ ] Task 0.3.4: 练习 String/Hash/List 基本命令
- [ ] Task 0.3.5: 理解向量索引概念
- [ ] Task 0.3.6: 练习 FT.CREATE 创建向量索引
- [ ] Task 0.3.7: 练习 FT.SEARCH 向量查询
- [ ] Task 0.3.8: 使用 Python redis 库连接 Redis

### Task 0.4: Go 项目初始化 (Day 22-28)

> Gin框架: https://gin-gonic.com/zh-cn/docs/quickstart/
> Go中文网: https://studygolang.com/pkgdoc

- [x] Task 0.4.1: 初始化 Go module (go mod init)
- [x] Task 0.4.2: 安装 Gin/Echo 框架
- [x] Task 0.4.3: 搭建基础 HTTP 服务骨架
- [x] Task 0.4.4: 配置日志库 (zap)
- [x] Task 0.4.5: 编写 config.yaml 配置文件
- [x] Task 0.4.6: 实现配置加载模块

---

## 二、开发阶段 (Phase 1) - Week 5-7

> OpenAI API: https://platform.openai.com/docs/api-reference/chat/create

### Task 1.1: OpenAI Provider 适配器 (Day 29-35)

- [x] Task 1.1.1: 定义请求/响应结构体 (OpenAI 格式)
- [x] Task 1.1.2: 安装 OpenAI Go SDK (使用直接 HTTP 调用替代)
- [x] Task 1.1.3: 实现 OpenAI API 调用
- [x] Task 1.1.4: 实现 /v1/chat/completions 路由
- [x] Task 1.1.5: 实现流式响应 (模拟 SSE)
- [ ] Task 1.1.6: 编写集成测试验证 API 调用

### Task 1.2: 请求/响应处理 (Day 36-42)

- [x] Task 1.2.1: 实现 messages 转 prompt 逻辑
- [x] Task 1.2.2: 实现响应格式化
- [x] Task 1.2.3: 实现错误处理和响应
- [x] Task 1.2.4: 添加日志记录
- [ ] Task 1.2.5: 编写单元测试

---

## 三、核心功能 (Phase 2) - Week 8-10

> TikToken Go: https://github.com/pkoukk/tiktoken-go
> Go限流: https://github.com/uber-go/ratelimit

### Task 2.1: 数据库集成 (Day 43-49)

- [ ] Task 2.1.1: 安装 PostgreSQL 或使用 MySQL
- [ ] Task 2.1.2: 设计 api_keys 表结构
- [x] Task 2.1.3: 实现数据库连接池
- [x] Task 2.1.4: 实现 API Key CRUD 操作
- [ ] Task 2.1.5: 编写数据库初始化 SQL

### Task 2.2: 认证鉴权 (Day 50-56)

- [x] Task 2.2.1: 实现 API Key 提取逻辑
- [x] Task 2.2.2: 实现 Key 校验中间件
- [x] Task 2.2.3: 实现 Key 权限检查
- [x] Task 2.2.4: 实现 Key 缓存 (Redis)
- [ ] Task 2.2.5: 编写认证测试

### Task 2.3: Token 限流 (Day 57-63)

- [ ] Task 2.3.1: 安装 tiktoken-go 库
- [ ] Task 2.3.2: 实现 Token 精确计算 (当前使用简单估算)
- [x] Task 2.3.3: 实现令牌桶算法
- [x] Task 2.3.4: 实现按模型限流 (框架)
- [x] Task 2.3.5: 实现按 API Key 限流 (框架)
- [ ] Task 2.3.6: 编写限流测试

### Task 2.4: L1 精确缓存 (Day 64-70)

- [x] Task 2.4.1: 实现 Redis 连接
- [x] Task 2.4.2: 实现 Hash 缓存键生成
- [x] Task 2.4.3: 实现 L1 缓存写入
- [x] Task 2.4.4: 实现 L1 缓存读取
- [ ] Task 2.4.5: 实现缓存 TTL
- [ ] Task 2.4.6: 编写缓存测试

### Task 2.5: 负载均衡与熔断 (Day 71-77)

- [ ] Task 2.5.1: 实现加权轮询算法
- [ ] Task 2.5.2: 实现 Provider 适配器接口
- [ ] Task 2.5.3: 实现 Claude Provider
- [ ] Task 2.5.4: 实现 MiniMax Provider
- [ ] Task 2.5.5: 实现熔断器 (连续失败 N 次熔断)
- [ ] Task 2.5.6: 实现自动降级逻辑

---

## 四、语义缓存 (Phase 3) - Week 11-12

> Sentence-Transformers: https://sbert.net/ (5分钟入门)
> HuggingFace: https://huggingface.co/sentence-transformers

### Task 3.1: Python Worker (Day 78-84)

- [ ] Task 3.1.1: 搭建 Python Worker FastAPI 服务
- [ ] Task 3.1.2: 安装 sentence-transformers
- [ ] Task 3.1.3: 下载 Embedding 模型 (all-MiniLM-L6-v2)
- [ ] Task 3.1.4: 实现 /embeddings 接口
- [ ] Task 3.1.5: 实现健康检查接口
- [ ] Task 3.1.6: 编写 Dockerfile

### Task 3.2: L2 语义缓存 (Day 85-91)

- [ ] Task 3.2.1: 创建 Redis Vector 索引
- [ ] Task 3.2.2: 实现 Embedding 生成调用 Python Worker
- [ ] Task 3.2.3: 实现向量相似度搜索
- [ ] Task 3.2.4: 实现 L2 缓存写入
- [ ] Task 3.2.5: 实现分层缓存逻辑 (L1 -> L2 -> LLM)
- [ ] Task 3.2.6: 编写缓存测试

---

## 五、运维部署 (Phase 4) - Week 13-14

> Docker Compose: https://docs.docker.com/compose/ | K8s: https://kubernetes.io/zh-cn/docs/tutorials/ | Prometheus: https://prometheus.io/docs/

### Task 4.1: Docker Compose (Day 92-98)

- [ ] Task 4.1.1: 编写 Gateway Dockerfile
- [ ] Task 4.1.2: 编写 Worker Dockerfile
- [ ] Task 4.1.3: 编写 docker-compose.yaml
- [ ] Task 4.1.4: 配置 Redis Stack 容器
- [ ] Task 4.1.5: 配置 PostgreSQL 容器
- [ ] Task 4.1.6: 本地联调测试

### Task 4.2: K8s 部署 (Day 99-105)

- [ ] Task 4.2.1: 学习 K8s 基础概念
- [ ] Task 4.2.2: 编写 Gateway Deployment
- [ ] Task 4.2.3: 编写 Worker Deployment
- [ ] Task 4.2.4: 编写 Service 配置
- [ ] Task 4.2.5: 编写 ConfigMap
- [ ] Task 4.2.6: 编写 HPA 配置
- [ ] Task 4.2.7: 部署到 K8s 集群

### Task 4.3: 监控集成 (Day 106-112)

- [ ] Task 4.3.1: 安装 Prometheus client
- [ ] Task 4.3.2: 实现自定义 Metrics
- [ ] Task 4.3.3: 配置 Prometheus 采集
- [ ] Task 4.3.4: 配置 Grafana 面板
- [x] Task 4.3.5: 实现健康检查接口

---

## 六、完善迭代 (Phase 5) - Week 15-16

### Task 5.1: Admin API (Day 113-119)

- [x] Task 5.1.1: 实现 Key 管理 API (CRUD)
- [x] Task 5.1.2: 实现模型管理 API
- [x] Task 5.1.3: 实现流量统计 API
- [x] Task 5.1.4: 添加权限控制

### Task 5.2: 测试与文档 (Day 120-126)

- [ ] Task 5.2.1: 编写单元测试 (覆盖率 > 60%)
- [ ] Task 5.2.2: 编写集成测试
- [x] Task 5.2.3: 编写 README 文档
- [ ] Task 5.2.4: 使用 k6 进行压测
- [ ] Task 5.2.5: 整理项目代码

---

## 七、代码里程碑 (速查)

| 里程碑 | 目标 | 对应 Task | 状态 |
|--------|------|-----------|------|
| M0 | Python + FastAPI + Redis Vector 会用 | Task 0.1 - 0.4 | - |
| M1 | Go HTTP 服务框架 | Task 0.4 | ✓ 完成 |
| M2 | 调用 OpenAI API | Task 1.1 | ✓ 完成 |
| M3 | 限流 + 鉴权 | Task 2.1 - 2.3 | 框架完成 |
| M4 | L1 缓存 | Task 2.4 | ✓ 完成 |
| M5 | L2 语义缓存 | Task 3.2 | - |
| M6 | K8s 部署 | Task 4.1 - 4.2 | - |

---

## 八、当前代码状态 (2026-02-17)

### 已实现的文件

```
llm-gateway/
├── cmd/server/main.go           # 主入口, Gin 路由, 中间件
├── internal/
│   ├── config/config.go         # 配置加载 (YAML)
│   ├── logger/logger.go         # Zap 日志库
│   ├── middleware/
│   │   ├── auth.go              # API Key 认证
│   │   └── ratelimit.go        # Token Bucket 限流
│   ├── handler/
│   │   ├── chat.go              # /v1/chat/completions
│   │   ├── embedding.go         # /v1/embeddings
│   │   └── admin.go            # Admin API (Key CRUD)
│   └── storage/
│       ├── redis.go             # Redis 客户端
│       └── postgres.go          # PostgreSQL 客户端
├── configs/config.yaml           # 配置文件
└── go.mod
```

### API 接口

- `GET /health` - 健康检查 ✓
- `POST /v1/chat/completions` - 聊天完成 (框架) ✓
- `POST /v1/embeddings` - 向量嵌入 ✓
- `GET /v1/models` - 模型列表 ✓
- `POST /api/v1/keys` - 创建 API Key ✓
- `GET /api/v1/keys` - 列表 API Keys ✓
- `DELETE /api/v1/keys/:id` - 删除 Key ✓
- `GET /api/v1/stats` - 使用统计 ✓

---

## 九、关键依赖版本

```
Go:
- Go 1.21+
- gin v1.9+
- go-redis v9+
- uber-go/zap
- gopkg.in/yaml.v3
- lib/pq (PostgreSQL)

Python:
- Python 3.11+
- fastapi v0.100+
- uvicorn v0.23+
- redis v5.0+
- sentence-transformers v2.2+
- numpy v1.24+

Docker:
- Docker Desktop 最新版
- Redis Stack 7.2+
- PostgreSQL 15+

K8s:
- Kubernetes 1.28+
```

---

*文档版本: v1.5*
*最后更新: 2026-02-17*
*代码扫描更新完成*
