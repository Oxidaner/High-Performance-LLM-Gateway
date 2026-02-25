# High-Performance LLM Gateway 开发计划

> 基于张世斌简历评估 | 项目周期: 10-14 周

---

## 代码状态总结 (2026-02-25)

### 已实现 ✅
| 模块 | 文件 | 状态 |
|------|------|------|
| HTTP 框架 | main.go | ✅ Gin + 健康检查 |
| 配置 | config/config.go | ✅ YAML 加载 |
| 日志 | middleware/logging.go | ✅ Zap 集成 |
| 认证 | middleware/auth.go | ✅ API Key |
| 限流 | middleware/ratelimit.go | ✅ Token Bucket (框架) |
| 存储 | storage/redis.go, postgres.go | ✅ 客户端 |
| Chat | handler/chat.go | ✅ 转发 OpenAI |
| Embedding | handler/embedding.go | ✅ 转发 Worker |
| Admin | handler/admin.go | ✅ Key CRUD + Stats |

### 未实现 ❌ (空文件)
| 模块 | 文件 | 状态 |
|------|------|------|
| L1 缓存 | service/cache/l1.go | ❌ 空 |
| L2 缓存 | service/cache/l2.go | ❌ 空 |
| 路由 | service/router.go | ❌ 空 |
| 熔断 | service/circuitbreaker.go | ❌ 空 |
| Claude Provider | service/provider/anthropic.go | ❌ 空 |
| MiniMax Provider | service/provider/minimax.go | ❌ 空 |
| TikToken | tokenizer/tiktoken.go | ❌ 空 |
| RAG | internal/rag/ | ❌ 不存在 |
| Agent | internal/agent/ | ❌ 不存在 |
| Python Worker | worker/ | ❌ 不存在 |

---

## Phase 1: 基础框架 ✅ (已完成)

### Task 1.1: OpenAI Provider 适配器
- [x] Task 1.1.1: 定义请求/响应结构体
- [x] Task 1.1.2: 安装 OpenAI Go SDK
- [x] Task 1.1.3: 实现 OpenAI API 调用
- [x] Task 1.1.4: 实现 /v1/chat/completions 路由
- [x] Task 1.1.5: 实现流式响应 (SSE)
- [ ] Task 1.1.6: 集成测试

### Task 1.2: 请求/响应处理
- [x] Task 1.2.1: messages 转 prompt
- [x] Task 1.2.2: 响应格式化
- [x] Task 1.2.3: 错误处理
- [x] Task 1.2.4: 日志记录
- [ ] Task 1.2.5: 单元测试

---

## Phase 2: 核心功能

### Task 2.1: 数据库集成
- [ ] Task 2.1.1: 安装 PostgreSQL
- [ ] Task 2.1.2: 设计 api_keys 表
- [x] Task 2.1.3: 数据库连接池
- [x] Task 2.1.4: Key CRUD 操作
- [ ] Task 2.1.5: 初始化 SQL

### Task 2.2: 认证鉴权
- [x] Task 2.2.1: API Key 提取
- [x] Task 2.2.2: Key 校验中间件
- [x] Task 2.2.3: Key 权限检查
- [x] Task 2.2.4: Key 缓存 (Redis)
- [ ] Task 2.2.5: 认证测试

### Task 2.3: Token 限流
- [ ] Task 2.3.1: 安装 tiktoken-go
- [ ] Task 2.3.2: Token 精确计算
- [x] Task 2.3.3: 令牌桶算法
- [x] Task 2.3.4: 按模型限流
- [x] Task 2.3.5: 按 Key 限流
- [ ] Task 2.3.6: 限流测试

### Task 2.4: L1 精确缓存
- [x] Task 2.4.1: Redis 连接
- [x] Task 2.4.2: Hash 缓存键生成
- [ ] Task 2.4.3: L1 缓存写入 (空文件)
- [ ] Task 2.4.4: L1 缓存读取 (空文件)
- [ ] Task 2.4.5: 缓存 TTL
- [ ] Task 2.4.6: 缓存测试

### Task 2.5: 负载均衡与熔断
- [ ] Task 2.5.1: 加权轮询算法 (空文件)
- [ ] Task 2.5.2: Provider 接口
- [ ] Task 2.5.3: Claude Provider (空文件)
- [ ] Task 2.5.4: MiniMax Provider (空文件)
- [ ] Task 2.5.5: 熔断器 (空文件)
- [ ] Task 2.5.6: 自动降级

### Task 2.6: 智能重试
- [ ] Task 2.6.1: 指数退避
- [ ] Task 2.6.2: 可重试错误判断
- [ ] Task 2.6.3: 最大重试次数
- [ ] Task 2.6.4: 熔断期跳过

### Task 2.7: Prompt 优化
- [ ] Task 2.7.1: 系统提示词缓存
- [ ] Task 2.7.2: 历史消息压缩
- [ ] Task 2.7.3: 上下文截断

### Task 2.8: 调用链观测
- [ ] Task 2.8.1: OpenTelemetry 集成
- [ ] Task 2.8.2: TraceID 生成透传
- [ ] Task 2.8.3: 关键节点埋点
- [ ] Task 2.8.4: Jaeger 上报

---

## Phase 3: 语义缓存

### Task 3.1: Python Worker
- [ ] Task 3.1.1: FastAPI 服务
- [ ] Task 3.1.2: sentence-transformers
- [ ] Task 3.1.3: Embedding 模型
- [ ] Task 3.1.4: /embeddings 接口
- [ ] Task 3.1.5: 健康检查
- [ ] Task 3.1.6: Dockerfile

### Task 3.2: L2 语义缓存
- [ ] Task 3.2.1: Redis Vector 索引
- [ ] Task 3.2.2: Embedding 调用
- [ ] Task 3.2.3: 向量相似度搜索
- [ ] Task 3.2.4: L2 缓存写入
- [ ] Task 3.2.5: 分层缓存逻辑
- [ ] Task 3.2.6: 缓存测试

---

## Phase 4: 运维部署

### Task 4.1: Docker Compose
- [ ] Task 4.1.1: Gateway Dockerfile
- [ ] Task 4.1.2: Worker Dockerfile
- [ ] Task 4.1.3: docker-compose.yaml
- [ ] Task 4.1.4: Redis Stack
- [ ] Task 4.1.5: PostgreSQL
- [ ] Task 4.1.6: 联调测试

### Task 4.2: K8s 部署
- [ ] Task 4.2.1: K8s 基础
- [ ] Task 4.2.2: Gateway Deployment
- [ ] Task 4.2.3: Worker Deployment
- [ ] Task 4.2.4: Service
- [ ] Task 4.2.5: ConfigMap
- [ ] Task 4.2.6: HPA
- [ ] Task 4.2.7: 集群部署

### Task 4.3: 监控
- [ ] Task 4.3.1: Prometheus client
- [ ] Task 4.3.2: 自定义 Metrics
- [ ] Task 4.3.3: Prometheus 采集
- [ ] Task 4.3.4: Grafana 面板
- [x] Task 4.3.5: 健康检查

---

## Phase 5: Admin API ✅

### Task 5.1: Admin API
- [x] Task 5.1.1: Key 管理 API
- [x] Task 5.1.2: 模型管理 API
- [x] Task 5.1.3: 流量统计 API
- [x] Task 5.1.4: 权限控制

### Task 5.2: 测试与文档
- [ ] Task 5.2.1: 单元测试
- [ ] Task 5.2.2: 集成测试
- [x] Task 5.2.3: README 文档
- [ ] Task 5.2.4: k6 压测
- [ ] Task 5.2.5: 代码整理

---

## Phase 6: AI Agent + RAG

### Task 6.1: RAG 基础
- [ ] Task 6.1.1: RAG 数据结构
- [ ] Task 6.1.2: 文档上传
- [ ] Task 6.1.3: 文本分块
- [ ] Task 6.1.4: Redis Vector 索引
- [ ] Task 6.1.5: 知识库 CRUD

### Task 6.2: RAG 检索
- [ ] Task 6.2.1: 向量相似度检索
- [ ] Task 6.2.2: RAG 问答接口
- [ ] Task 6.2.3: 上下文组装
- [ ] Task 6.2.4: 相似度阈值

### Task 6.3: Agent 框架
- [ ] Task 6.3.1: Agent 核心结构
- [ ] Task 6.3.2: ReAct 推理
- [ ] Task 6.3.3: 工具注册表
- [ ] Task 6.3.4: Agent API 接口

### Task 6.4: Agent 工具
- [ ] Task 6.4.1: 网络搜索工具
- [ ] Task 6.4.2: 数据库查询工具
- [ ] Task 6.4.3: 其他工具
- [ ] Task 6.4.4: 工具注册发现

### Task 6.5: 优化集成
- [ ] Task 6.5.1: 边缘场景
- [ ] Task 6.5.2: 优化与测试

---

## 里程碑

| 里程碑 | 状态 | 说明 |
|--------|------|------|
| M0 | ✅ | 学习阶段 |
| M1 | ✅ | Go HTTP 框架 |
| M2 | ✅ | OpenAI API 调用 |
| M3 | ⚠️ | 限流+鉴权 (框架有，精确计算无) |
| M4 | ❌ | L1 缓存 (空文件) |
| M5 | ❌ | L2 语义缓存 (空文件) |
| M6 | ❌ | K8s 部署 |
| M7 | ❌ | RAG |
| M8 | ❌ | Agent |
| M9 | ❌ | Agent 工具 |

---

*最后更新: 2026-02-25*
