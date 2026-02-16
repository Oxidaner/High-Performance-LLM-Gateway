- # High-Performance LLM Gateway 规格说明书

  ## 1. 项目概述

  ### 1.1 项目简介
  面向企业级大模型调用场景的高性能流量网关，旨在解决模型调用成本高、延迟不稳定及供应商锁定问题。

  ### 1.2 项目定位
  - **项目类型**: 个人项目 / Side Project
  - **部署方式**: Kubernetes
  - **技术栈**: Go (API 网关) + Python (AI 任务处理)

  ### 1.3 核心特性
  | 特性       | 描述                                | 预期收益                            |
  | ---------- | ----------------------------------- | ----------------------------------- |
  | 语义缓存   | Redis Vector 语义缓存，相似度 >0.95 | 降低 API 成本 40%，P99 延迟降低 60% |
  | 多模型路由 | 加权轮询 + 自动熔断降级             | 高可用 + 成本优化                   |
  | Token 限流 | 令牌桶算法，Tokenizer 精确预估      | 10k+ QPS 稳定运行                   |

  ### 1.4 性能目标
  - **吞吐量**: 10,000+ QPS
  - **P99 延迟**: < 500ms
  - **可用性**: 99.9%

  ---

  ## 2. 功能需求

  ### 2.1 LLM 提供商集成

  | 提供商             | 支持状态 | 优先级 |
  | ------------------ | -------- | ------ |
  | OpenAI (GPT-4/3.5) | ✅ 支持   | P0     |
  | Claude (Anthropic) | ✅ 支持   | P0     |
  | minimax            | ✅ 支持   | P1     |

  #### 2.1.1 接口统一封装
  - 统一请求/响应格式
  - 错误码标准化
  - 支持流式输出 (SSE)

  ### 2.2 分层缓存系统

  > ⚠️ **性能优化**: 采用 L1 + L2 分层缓存，避免每次请求都走 Embedding 计算

  #### 2.2.1 分层缓存架构

  | 层级   | 缓存类型 | 实现方式                   | 延迟       | 命中率预估 |
  | ------ | -------- | -------------------------- | ---------- | ---------- |
  | **L1** | 精确缓存 | Redis Hash (SHA256 prompt) | < 1ms      | 60-80%     |
  | **L2** | 语义缓存 | Redis Vector (FT.SEARCH)   | 10-50ms    | 10-20%     |
  | **L0** | 无缓存   | 直连 LLM                   | 500-3000ms | -          |

  #### 2.2.2 L1 精确缓存
  - **缓存键**: `SHA256(prompt + model + temperature)`
  - **用途**: 拦截高频重复请求 (如轮询场景、相同 Prompt)
  - **特点**: 微秒级查询，无需调用 Python Worker

  #### 2.2.3 L2 语义缓存
  - **向量模型**: text-embedding-ada-002 / text-embedding-3-small
  - **相似度阈值**: > 0.95
  - **用途**: 处理语义相似但文本不同的请求

  #### 2.2.4 缓存流程 (优化后)
  ```
  请求文本
       │
       ▼
  ┌────────────────┐
  │ L1 精确缓存     │
  │ Hash 查询 Redis│
  └────────────────┘
       │
       ├─ 命中 (L1 Hit) → 直接返回 (< 1ms, 最快路径)
       │
       └─ 未命中
             │
             ▼
       ┌─────────────────┐
       │ L2 语义缓存      │
       │ 1. Python Worker│
       │    生成 Embedding│
       │ 2. Redis Vector│
       │    FT.SEARCH   │
       └─────────────────┘
             │
             ├─ 命中 (L2 Hit) → 返回缓存 (10-50ms)
             │
             └─ 未命中 → Token 计数 & 限流 → LLM 调用
  ```

  ```mermaid
  flowchart TD
      Start["请求文本"] --> L1["L1 精确缓存<br/>Hash(prompt) Redis"]
      L1 -->|命中| L1_Hit["直接返回<br/>< 1ms"]
      L1 -->|未命中| L2["L2 语义缓存<br/>Python Worker Embedding"]
      L2 --> Vector["Redis FT.SEARCH<br/>向量相似度"]
      Vector -->|相似度 > 0.95| L2_Hit["返回缓存<br/>10-50ms"]
      Vector -->|未命中| Token["Token 计算<br/>Go 内置 TikToken"]
      Token --> Rate["令牌桶限流"]
      Rate -->|通过| LLM["调用 LLM"]
      Rate -->|拒绝| Reject["HTTP 429"]
      
      L1_Hit -.->|缓存内容| Client
      L2_Hit -.->|缓存内容| Client
      LLM -.->|响应| Client
      
      style L1 fill:#c8e6c9,stroke:#2e7d32
      style L2 fill:#fff9c4,stroke:#f57f17
      style Token fill:#bbdefb,stroke:#1565c0
      style LLM fill:#ffccbc,stroke:#d84315
  ```

  > **面试话术**: "我采用了分层缓存策略，L1 使用 Hash 实现微秒级精确匹配，拦截 80% 的高频重复请求；L2 处理语义相似请求。这样避免每次都进行 Embedding 计算，降低了 P95 延迟。"

  #### 2.2.5 缓存命中时的流式处理
  | 场景               | 处理方式                                                     |
  | ------------------ | ------------------------------------------------------------ |
  | 非流式请求命中缓存 | 直接返回完整响应                                             |
  | 流式请求命中缓存   | **模拟 SSE 流式行为**，将完整文本拆分为 chunk 发送 (用户体验一致) |
  | 流式切分策略       | 按句子/段落切分，每 20-50 字符一个 chunk，模拟真实 LLM 流式输出 |

  #### 2.2.6 缓存配置
  | 参数                 | 默认值  | 说明                          |
  | -------------------- | ------- | ----------------------------- |
  | l1_enabled           | true    | L1 精确缓存开关               |
  | l2_enabled           | true    | L2 语义缓存开关               |
  | l1_ttl               | 1 hour  | L1 缓存过期时间 (短,高频数据) |
  | l2_ttl               | 7 days  | L2 缓存过期时间               |
  | similarity_threshold | 0.95    | L2 相似度阈值                 |
  | max_cache_size       | 100,000 | 最大缓存条目                  |

  ### 2.3 多模型负载均衡

  #### 2.3.1 路由策略
  - **主要策略**: 加权轮询 (Weighted Round Robin)
  - **权重配置**: 按模型/提供商配置
  - **故障转移**: 自动熔断 + 降级

  #### 2.3.2 熔断降级
  | 状态          | 处理策略           |
  | ------------- | ------------------ |
  | 连续 3 次失败 | 熔断 30 秒         |
  | 熔断期间      | 自动切换到备用模型 |
  | 恢复检测      | 成功后自动恢复     |

  #### 2.3.3 模型配置示例
  ```yaml
  models:
    - name: gpt-4
      provider: openai
      weight: 5
      fallback: gpt-3.5-turbo
      
    - name: gpt-3.5-turbo
      provider: openai
      weight: 3
      fallback: claude-3-haiku
      
    - name: claude-3-haiku
      provider: anthropic
      weight: 2
  ```

  ### 2.4 Token 精确流控

  #### 2.4.1 限流算法
  - **算法**: 令牌桶 (Token Bucket)
  - **Token 计算**: Go 网关层内置 TikToken，**避免 RPC 调用**
    - **OpenAI 模型**: `pkoukk/tiktoken-go` 精确计算 (< 1ms)
    - **非 OpenAI 模型**: 字符数 × 系数估算 (Trade-off)
  - **粒度**: 全局限流 + 按模型限流 + 按 API Key 限流

  #### 2.4.2 Tokenizer 配置 (Go 内置)
  > ⚠️ **性能优化**: Token 计算在 Go 进程内完成，避免每次请求都跨进程调用 Python

  | 模型类型           | Tokenizer        | 计算方式 | 精度 |
  | ------------------ | ---------------- | -------- | ---- |
  | OpenAI (GPT-4/3.5) | tiktoken-go (Go) | 精确计算 | ±2%  |
  | Claude (Anthropic) | 字符数 × 0.75    | 估算     | ±10% |
  | 通义千问/MiniMax   | 字符数 × 0.6     | 估算     | ±15% |

  > **面试话术**: "为了保证 10k QPS 的性能目标，Token 计算在 Go 网关层直接完成，使用 TikToken 的 Go 移植版本。对于非 OpenAI 模型采用字符数估算，这是一个典型的工程 Trade-off。"

  #### 2.4.3 限流流程
  ```
  请求 → Go Gateway (内置 TikToken)
          │
          ▼
     Token 计算 (< 1ms)
          │
          ▼
     令牌桶检查 (限流/拦截)
          │
          ▼
     上下文长度检查 (max_tokens > model.max_context?)
          │
          ├─ 超长 → HTTP 400 返回
          │
          └─ 正常 → 转发 LLM
  ```

  #### 2.4.3 限流配置
  | 参数        | 默认值     | 说明             |
  | ----------- | ---------- | ---------------- |
  | global_rate | 10,000 QPS | 全局 QPS 限制    |
  | model_rate  | 5,000 QPS  | 单模型 QPS 限制  |
  | burst_size  | 500        | 突发容量         |
  | max_tokens  | 128,000    | 单请求最大 token |

  #### 2.4.4 长文本保护与上下文截断
  > ⚠️ **网关层前置拦截**: 在网关层检测并拒绝超长请求，节省 Token 成本

  - **前置检查**: Go Gateway 内置 TikToken 计算请求 token 数 (< 1ms)
  - **超长拦截**: 请求 token > 模型 max_context 时，**网关直接返回错误**，不调用 LLM
  - **错误响应**: 返回 OpenAI 兼容格式的 error message
  - **拦截收益**: 避免浪费用户配额 + 节省 LLM API 成本

  | 场景                    | 处理方式                         |
  | ----------------------- | -------------------------------- |
  | token > max_context     | HTTP 400 + "max_tokens exceeded" |
  | token > 128k (绝对上限) | HTTP 400 + "request too large"   |
  | 正常请求                | 放行至 LLM                       |

  #### 2.4.5 异常处理标准化
  > ⚠️ **统一封装**: 将各供应商的非标准错误转换为 OpenAI 格式

  | LLM 返回状态码 | 原始错误            | 封装后错误 (OpenAI 格式)                                     |
  | -------------- | ------------------- | ------------------------------------------------------------ |
  | 429            | Rate Limit          | `{"error": {"type": "rate_limit_error", "message": "..."}}`  |
  | 500            | Server Error        | `{"error": {"type": "server_error", "message": "..."}}`      |
  | 401            | Auth Failed         | `{"error": {"type": "invalid_api_key", "message": "..."}}`   |
  | 403            | Permission Denied   | `{"error": {"type": "permission_error", "message": "..."}}`  |
  | 503            | Service Unavailable | `{"error": {"type": "service_unavailable", "message": "..."}}` |

  > **面试点**: 异常标准化是网关的核心价值之一，确保客户端感知一致

  ### 2.5 API 接口

  #### 2.5.1 对外 API (OpenAI 兼容)
  ```
  POST /v1/chat/completions      # 聊天完成
  POST /v1/completions           # 文本完成
  POST /v1/embeddings            # 向量嵌入
  GET  /v1/models                # 模型列表
  ```

  #### 2.5.2 管理 API
  ```
  POST   /api/v1/keys            # 创建 API Key
  GET    /api/v1/keys            # 获取 Key 列表
  DELETE /api/v1/keys/:id        # 删除 Key
  GET    /api/v1/stats           # 流量统计
  POST   /api/v1/models          # 添加模型
  PUT    /api/v1/models/:id      # 更新模型配置
  ```

  #### 2.5.3 请求示例
  ```bash
  # 聊天完成
  curl -X POST http://localhost:8080/v1/chat/completions \
    -H "Authorization: Bearer sk-xxxx" \
    -H "Content-Type: application/json" \
    -d '{
      "model": "gpt-4",
      "messages": [{"role": "user", "content": "Hello!"}],
      "stream": false
    }'
  ```

  #### 2.5.4 错误响应格式 (OpenAI 兼容)
  ```json
  {
    "error": {
      "message": "Error message description",
      "type": "invalid_request_error",
      "code": "invalid_api_key"
    }
  }
  ```

  #### 2.5.5 错误码定义
  | HTTP 状态码 | error.type            | error.code          | 说明                      | 排查方向             |
  | ----------- | --------------------- | ------------------- | ------------------------- | -------------------- |
  | 400         | invalid_request_error | max_tokens_exceeded | 请求 token 超过模型上下文 | 检查 max_tokens 参数 |
  | 400         | invalid_request_error | request_too_large   | 请求体过大                | 压缩 prompt          |
  | 401         | invalid_api_key       | invalid_api_key     | API Key 无效              | 检查 Key 是否正确    |
  | 401         | invalid_api_key       | key_expired         | API Key 已过期            | 续期 Key             |
  | 403         | permission_error      | key_disabled        | API Key 已禁用            | 启用 Key             |
  | 429         | rate_limit_error      | rate_limit_exceeded | 触发限流                  | 降低请求频率         |
  | 429         | rate_limit_error      | quota_exceeded      | 配额耗尽                  | 充值/联系管理员      |
  | 500         | server_error          | internal_error      | LLM 服务内部错误          | 重试/切换模型        |
  | 502         | server_error          | bad_gateway         | 模型服务商网关错误        | 切换模型             |
  | 503         | service_unavailable   | model_overloaded    | 模型过载                  | 降级/重试            |
  | 503         | service_unavailable   | model_not_available | 模型暂不可用              | 切换模型             |

  #### 2.5.6 熔断错误处理
  | 状态         | 响应                           | 处理策略               |
  | ------------ | ------------------------------ | ---------------------- |
  | 模型熔断中   | 503 + "model_circuit_breaker"  | 自动切换 fallback 模型 |
  | 所有模型熔断 | 503 + "all_models_unavailable" | 返回降级响应           |

  ### 2.6 认证授权

  #### 2.6.1 认证方式
  - **API Key**: 简单场景
  - **OAuth2**: 企业场景 (可选)

  #### 2.6.2 Key 管理
  - Key 格式: `sk-` 前缀 + 32 位随机字符串
  - 支持设置 Key 有效期
  - 支持设置 Key 速率限制

  #### 2.6.3 配置热更新机制
  > ⚠️ **轻量化方案**: K8s ConfigMap + fsnotify 热更新，无需额外中间件

  | 配置项       | 热更新方式                  | 生效时间      |
  | ------------ | --------------------------- | ------------- |
  | 模型权重     | fsnotify 监听 + 内存 reload | < 1s          |
  | 限流参数     | fsnotify 监听 + 内存 reload | < 1s          |
  | 缓存阈值     | fsnotify 监听 + 内存 reload | < 1s          |
  | 新增 API Key | DB 写入 + Redis 缓存刷新    | 5s (缓存 TTL) |

  ```
  K8s ConfigMap 变更
          │
          ▼
     fsnotify 事件触发
          │
          ├── reload_models()      → 更新内存中的模型配置
          ├── reload_ratelimit()   → 重置令牌桶
          └── reload_cache()       → 更新缓存阈值
  ```

  > **面试话术**: "考虑到个人项目的轻量化需求，我选择了 K8s ConfigMap + fsnotify 的方案。相比 Nacos，这个方案零额外依赖，同时利用 K8s 原生特性，面试时可以展示对 K8s 的理解。"

  ### 2.7 Admin 管理后台

  #### 2.7.1 功能模块
  | 模块     | 功能                                 |
  | -------- | ------------------------------------ |
  | 仪表盘   | 实时 QPS、延迟、缓存命中率、成本统计 |
  | 模型管理 | 添加/编辑/删除模型，配置权重         |
  | Key 管理 | 创建/禁用/删除 API Key               |
  | 流量分析 | 请求日志、错误统计、趋势图           |
  | 系统配置 | 限流参数、缓存配置、告警阈值         |

  #### 2.7.2 技术选型
  - **前端**: React + Ant Design
  - **图表**: Recharts / ECharts

  ---

  ## 3. 技术架构

  ### 3.1 系统架构图

  ┌─────────────────────────────────────────────────────────────────┐
  │                         Kubernetes                               │
  │                                                                 │
  │  ┌─────────────────────────────────────────────────────────────┐
  │  │                    Go Gateway (Deployment)                   │
  │  │                      (:8080, 高性能)                          │
  │  └────────┬──────────────────────┬──────────────────────┬──────┘
  │           │                      │                      │
  │           ▼                      ▼                      ▼
  │  ┌──────────────┐    ┌──────────────────┐    ┌──────────────┐
  │  │  PostgreSQL  │    │  Python Worker   │    │  Redis Stack │
  │  │  (持久化)     │    │  (独立 Deployment)│    │  (缓存/向量)  │
  │  │  - API Keys  │    │  (:8081)         │    │              │
  │  │  - 模型配置   │    │  - Embedding     │    │              │
  │  │  - 请求日志   │    │  - Token 计算   │    │              │
  │  └──────────────┘    └────────┬─────────┘    └──────────────┘
  │                                │                      │
  │                                └──────────┬───────────┘
  │                                           │
  │                                           ▼
  │                        ┌──────────────────────────────────────┐
  │                        │         LLM Providers                │
  │                        │  ┌────────┐ ┌───────┐ ┌─────────┐  │
  │                        │  │ OpenAI │ │Claude │ │ Minimax │  │
  │                        │  └────────┘ └───────┘ └─────────┘  │
  │                        └──────────────────────────────────────┘
  └─────────────────────────────────────────────────────────────────┘

  ┌─────────────────────────────────────────────────────────────────┐
  │                    监控/日志/配置 (K8s 集群外)                    │
  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
  │  │Prometheus│  │ Grafana  │  │  Jaeger  │  │K8s CM   │       │
  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘       │
  │                                                                 │
  │  ┌──────────────────────────────────────────────────────────┐  │
  │  │                     ELK/EFK (日志收集)                      │  │
  │  └──────────────────────────────────────────────────────────┘  │
  └─────────────────────────────────────────────────────────────────┘

  

  ```mermaid
  flowchart TB
      subgraph K8s["Kubernetes Cluster"]
          subgraph Gateway["Go Gateway (Deployment)"]
              G[":8080<br/>高性能HTTP"]
          end
          
          subgraph Worker["Python Worker (独立 Deployment)"]
              P[":8081<br/>Embedding"]
          end
          
          subgraph Data["Data Layer"]
              Redis["Redis Stack<br/>向量+缓存+限流"]
              DB["PostgreSQL<br/>API Keys+配置+日志"]
          end
          
          subgraph LLM["LLM Providers"]
              OpenAI["OpenAI<br/>GPT-4/3.5"]
              Claude["Claude<br/>Anthropic"]
              MiniMax["MiniMax"]
          end
      end
      
      subgraph Monitor["监控/日志 (集群外)"]
          Prom["Prometheus"]
          Graf["Grafana"]
          Jaeger["Jaeger"]
          K8sCM["K8s ConfigMap"]
          ELK["ELK/EFK"]
      end
      
      Client --> G
      G -->|L1 精确缓存| Redis
      G -->|L2 语义缓存| P
      P -->|向量搜索| Redis
      G -->|鉴权/持久化| DB
      G -->|加权轮询| OpenAI
      G -->|加权轮询| Claude
      G -->|加权轮询| MiniMax
      
      G -->|Metrics| Prom
      Prom --> Graf
      G -->|Trace| Jaeger
      G -->|Config| K8sCM
      G -->|Logs| ELK
      
      style Gateway fill:#e1f5fe,stroke:#01579b
      style Worker fill:#fce4ec,stroke:#c2185b
      style Data fill:#e8f5e9,stroke:#2e7d32
      style LLM fill:#fff3e0,stroke:#ef6c00
      style Monitor fill:#f3e5f5,stroke:#7b1fa2
  ```

  ### 3.2 组件职责

  #### 3.2.1 Go Gateway (独立 Deployment)
  - HTTP/REST API 服务 (处理 10k QPS)
  - 请求路由与负载均衡
  - Token 限流 (令牌桶)
  - 认证授权 (API Key 校验)
  - 指标采集
  - **不直接做 Embedding 计算，调用 Python Worker**

  #### 3.2.2 Python Worker (独立 Deployment)
  > ⚠️ **重要设计决策**: 不使用 Sidecar 模式，独立部署
  > - 原因: Embedding/Token 计算是 CPU 密集型，会抢占 Go Gateway 的 CPU 资源
  > - 部署: 独立 Deployment，通过 Service 内网调用

  - 向量 Embedding 生成
  - 复杂 Token 计算 (非 OpenAI 模型)

  #### 3.2.2.1 故障降级策略 (优雅降级)
  > ⚠️ **系统韧性**: Python Worker 不是强依赖，是"优化依赖"

  | 故障场景             | 降级策略                       | 影响                         |
  | -------------------- | ------------------------------ | ---------------------------- |
  | Python Worker 不可用 | 跳过 L2 语义缓存，直接透传 LLM | 缓存命中率 ↓，但业务不断     |
  | Embedding 超时 (>5s) | 降级为 L1 精确缓存             | 语义缓存失效，精确缓存仍有效 |
  | Worker OOM/崩溃      | 自动摘除流量，恢复后自动加入   | 短暂影响，后续请求走 LLM     |

  ```go
  // Go Gateway 伪代码: 优雅降级
  func (g *Gateway) handleCache(ctx context.Context, prompt string) (*CacheResult, error) {
      // L1 精确缓存 (不依赖 Python)
      if result := g.l1Cache.Get(prompt); result != nil {
          return result, nil
      }
      
      // L2 语义缓存 (依赖 Python Worker)
      embedding, err := g.pythonClient.GetEmbedding(ctx, prompt)
      if err != nil {
          // 故障降级: 跳过 L2，直接走 LLM
          log.Warn("Python Worker unavailable, skipping L2 cache", "error", err)
          return nil, ErrL2CacheMiss
      }
      
      // ... L2 查找逻辑
  }
  
  // Python Worker 健康检查
  // - 定期 ping 检查可用性
  // - 连续 3 次失败则摘除流量
  // - 成功后自动恢复
  ```

  > **面试话术**: "考虑到 Python Worker 承载了不稳定的 AI 推理任务，我在 Go 网关层做了优雅降级设计。当 Worker 不可用时，系统会自动降级为'直连模式'，牺牲缓存命中率，但保证核心业务链路不中断。"

  #### 3.2.3 PostgreSQL (持久化存储)
  > ⚠️ **核心数据必须持久化**: Redis 是内存数据库，重启会丢失所有数据

  | 表名         | 用途            | 核心字段                                                     |
  | ------------ | --------------- | ------------------------------------------------------------ |
  | api_keys     | API Key 管理    | key, key_hash, name, rate_limit, expires_at, is_active       |
  | models       | 模型配置        | name, provider, weight, fallback, max_context, tokenizer, is_active |
  | request_logs | 请求日志        | key_id, model, tokens, latency, cost, created_at             |
  | users        | 用户管理 (预留) | email, role, created_at                                      |

  ```sql
  -- API Keys 表
  CREATE TABLE api_keys (
      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      key_hash VARCHAR(64) NOT NULL UNIQUE,  -- SHA256 hash of API key
      name VARCHAR(255),
      rate_limit INTEGER DEFAULT 1000,
      is_active BOOLEAN DEFAULT true,
      expires_at TIMESTAMP,
      created_at TIMESTAMP DEFAULT NOW(),
      updated_at TIMESTAMP DEFAULT NOW()
  );
  
  -- Models 表
  CREATE TABLE models (
      id SERIAL PRIMARY KEY,
      name VARCHAR(255) NOT NULL UNIQUE,
      provider VARCHAR(50) NOT NULL,
      weight INTEGER DEFAULT 1,
      fallback VARCHAR(255),
      max_context INTEGER DEFAULT 8192,
      tokenizer VARCHAR(50) DEFAULT 'tiktoken',
      is_active BOOLEAN DEFAULT true,
      created_at TIMESTAMP DEFAULT NOW()
  );
  
  -- Request Logs 表 (可按需分表/分区)
  CREATE TABLE request_logs (
      id BIGSERIAL PRIMARY KEY,
      key_id UUID REFERENCES api_keys(id),
      model VARCHAR(255),
      prompt_tokens INTEGER,
      completion_tokens INTEGER,
      latency_ms INTEGER,
      cost DECIMAL(10, 6),
      status VARCHAR(20),
      created_at TIMESTAMP DEFAULT NOW()
  );
  
  -- 索引优化 (面试加分项)
  CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);           -- API Key 校验加速
  CREATE INDEX idx_api_keys_is_active ON api_keys(is_active);          -- 活跃 Key 查询
  CREATE INDEX idx_request_logs_created_at ON request_logs(created_at); -- 时间范围查询
  CREATE INDEX idx_request_logs_key_id ON request_logs(key_id);       -- 用户维度统计
  CREATE INDEX idx_request_logs_model ON request_logs(model);         -- 模型维度统计
  CREATE INDEX idx_request_logs_status ON request_logs(status);       -- 错误分析
  
  -- 大表分区 (数据量大时)
  -- ALTER TABLE request_logs PARTITION BY RANGE (created_at);
  ```

  #### 3.2.4 Redis Stack
  - 向量存储与检索 (FT.SEARCH)
  - API 响应缓存 (热点数据)
  - 限流计数器
  - 分布式锁
  - **Key 权限缓存** (加速鉴权，5 分钟 TTL)

  #### 3.2.5 Redis 内存治理
  > ⚠️ **面试加分项**: 防止内存溢出，设置合理的淘汰策略

  | 配置项           | 值          | 说明                                     |
  | ---------------- | ----------- | ---------------------------------------- |
  | maxmemory        | 4GB         | 根据业务量调整                           |
  | maxmemory-policy | allkeys-lru | 优先淘汰最近最少使用的 Key               |
  | L1 缓存 TTL      | 1 hour      | 精确缓存时效短，LRU 友好                 |
  | L2 缓存 TTL      | 7 days      | 向量缓存重要，但内存不足时淘汰旧数据合理 |

  ```bash
  # Redis 配置
  redis-server --maxmemory 4gb --maxmemory-policy allkeys-lru
  ```

  > **面试话术**: "Redis 内存治理采用 allkeys-lru 策略。L1 精确缓存时效短，适合淘汰；L2 向量缓存虽然重要，但在内存压力下淘汰旧向量是合理的权衡，避免服务崩溃。"

  ### 3.3 数据流 (完整链路)

  ```
  Client Request
       │
       ▼
  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
  │  Auth Check │───▶│ Rate Limit  │───▶│   Router    │
  │ (Redis缓存)  │    │ (令牌桶)    │    │             │
  └─────────────┘    └─────────────┘    └─────────────┘
                                             │
                                             ▼
                               ┌─────────────────────────────┐
                               │     语义缓存检查             │
                               │  1. 提取 prompt             │
                               │  2. 调用 Python Worker      │
                               │     生成 Embedding          │
                               │  3. Redis FT.SEARCH         │
                               │  4. 相似度 > 0.95?          │
                               └─────────────────────────────┘
                                │                    │
                           YES  │                    │  NO
                                ▼                    ▼
                      ┌────────────────┐    ┌─────────────────┐
                      │  命中缓存       │    │  LLM 调用       │
                      │  (流式模拟)    │    │  (加权轮询)     │
                      └────────────────┘    └─────────────────┘
                                │                    │
                                └────────┬──────────┘
                                         ▼
                                ┌─────────────────┐
                                │  存入向量缓存   │
                                │  (可选)         │
                                └─────────────────┘
                                         │
                                         ▼
                                ┌─────────────────┐
                                │   Response      │
                                │   to Client     │
                                └─────────────────┘
  ```

  ```mermaid
  sequenceDiagram
      participant Client
      participant Go as Go Gateway
      participant Redis as Redis Stack
      participant Python as Python Worker
      participant LLM as LLM Providers
      
      Client->>Go: POST /v1/chat/completions
      Go->>Go: Auth Check (API Key)
      Go->>Redis: GET api_key
      Redis-->>Go: Key Info (cached)
      
      Go->>Go: Rate Limit (Token Bucket)
      
      Note over Go: L1 精确缓存
      Go->>Redis: HGET cache:l1 {hash(prompt)}
      alt L1 Hit
          Redis-->>Go: Cached Response
          Go-->>Client: Return (fastest)
      else L1 Miss
          Note over Go: L2 语义缓存
          Go->>Python: Get Embedding
          alt Python Available
              Python-->>Go: Embedding Vector
              Go->>Redis: FT.SEARCH vector_index
              alt L2 Hit
                  Redis-->>Go: Cached Response
                  Go-->>Client: Return (simulate stream)
              else L2 Miss
                  Go->>Go: Token Count (TikToken)
                  Go->>LLM: Forward Request
                  LLM-->>Go: Response
                  Go->>Redis: SET cache:l2 {embedding, response}
                  Go-->>Client: Response
              end
          else Python Unavailable
              Note over Go: 优雅降级
              Go->>LLM: Direct Forward
              LLM-->>Go: Response
              Go-->>Client: Response
          end
      end
  ```

  ### 3.4 认证流程 (含缓存)
  ```
  请求 → Extract API Key → Redis GET(key) → 命中 → 校验权限
                            │              │
                            └─ 未命中 ──────▶ PostgreSQL GET
                                             │
                                             ▼
                                        权限校验
                                             │
                                             ▼
                                        存入 Redis (TTL 5min)
  ```

  ---

  ## 4. 技术选型

  ### 4.1 核心技术栈

  | 组件           | 技术              | 版本      | 说明                      |
  | -------------- | ----------------- | --------- | ------------------------- |
  | Gateway        | Go                | 1.21+     | 高性能 HTTP 服务          |
  | AI 任务        | Python            | 3.11+     | Token 计算/Embedding      |
  | 向量存储       | Redis Stack       | 7.2+      | 向量搜索 + 缓存           |
  | **持久化存储** | **PostgreSQL**    | **15+**   | **API Key/模型配置/日志** |
  | **配置中心**   | **K8s ConfigMap** | **1.28+** | **配置热更新 (fsnotify)** |
  | 监控           | Prometheus        | 2.45+     | Metrics 采集              |
  | 可视化         | Grafana           | 10.0+     | 监控面板                  |
  | 链路追踪       | Jaeger            | 1.47+     | 分布式追踪                |
  | 日志           | ELK/EFK           | 8.x       | 日志收集分析              |
  | K8s            | Kubernetes        | 1.28+     | 容器编排                  |

  ### 4.2 Go 依赖
  | 包                       | 用途                          |
  | ------------------------ | ----------------------------- |
  | gin / echo               | HTTP 框架                     |
  | redis / go-redis         | Redis 客户端                  |
  | uber-go/ratelimit        | 令牌桶限流                    |
  | pkoukk/tiktoken-go       | Token 精确计算 (内置，无 RPC) |
  | prometheus/client_golang | 监控                          |
  | jaeger-client-go         | 链路追踪                      |
  | lib/pq                   | PostgreSQL 驱动               |

  ### 4.3 Python 依赖
  | 包                    | 用途                |
  | --------------------- | ------------------- |
  | sentence-transformers | 向量 Embedding 生成 |
  | fastapi               | HTTP 服务           |
  | redis                 | Redis 客户端        |

  ### 4.4 代码目录结构

  #### 4.4.1 Go 项目结构
  ```
  llm-gateway/
  ├── cmd/
  │   └── server/
  │       └── main.go              # 入口文件
  ├── internal/
  │   ├── config/
  │   │   └── config.go            # 配置加载 (fsnotify 热更新)
  │   ├── handler/
  │   │   ├── chat.go              # /v1/chat/completions
  │   │   ├── embedding.go         # /v1/embeddings
  │   │   └── admin.go             # /api/v1 管理接口
  │   ├── middleware/
  │   │   ├── auth.go              # API Key 鉴权
  │   │   ├── ratelimit.go         # 令牌桶限流
  │   │   └── logging.go           # 请求日志
  │   ├── service/
  │   │   ├── router.go            # 负载均衡/路由
  │   │   ├── cache/
  │   │   │   ├── l1.go            # L1 精确缓存
  │   │   │   └── l2.go            # L2 语义缓存
  │   │   ├── provider/
  │   │   │   ├── openai.go        # OpenAI 适配器
  │   │   │   ├── anthropic.go     # Claude 适配器
  │   │   │   └── minimax.go       # MiniMax 适配器
  │   │   └── circuitbreaker.go    # 熔断器
  │   ├── tokenizer/
  │   │   └── tiktoken.go          # Go 内置 TikToken
  │   ├── model/
  │   │   └── model.go             # 模型定义
  │   └── storage/
  │       ├── redis.go             # Redis 客户端
  │       └── postgres.go           # PostgreSQL 客户端
  ├── pkg/
  │   └── errors/
  │       └── errors.go             # 错误定义
  ├── configs/
  │   └── config.yaml              # 配置文件
  ├── deployments/
  │   ├── k8s/
  │   │   ├── deployment.yaml       # K8s Deployment
  │   │   ├── service.yaml
  │   │   ├── hpa.yaml
  │   │   └── configmap.yaml       # K8s ConfigMap
  │   └── docker/
  │       └── docker-compose.yaml
  ├── scripts/
  │   └── init_db.sql              # 数据库初始化
  ├── go.mod
  └── go.sum
  ```

  #### 4.4.2 Python 项目结构
  ```
  llm-worker/
  ├── app/
  │   ├── main.py                  # FastAPI 入口
  │   ├── routes/
  │   │   ├── embedding.py         # 向量生成 API
  │   │   └── health.py            # 健康检查
  │   └── services/
  │       └── embedding_service.py # Embedding 逻辑
  ├── models/
  │   └── cache.py                 # 模型缓存
  ├── configs/
  │   └── config.yaml
  ├── requirements.txt
  └── Dockerfile
  ```

  > **设计原则**:
  > - `internal/` 对外不可见，只暴露 `handler` 层
  > - `provider` 适配器模式，方便扩展新 LLM
  > - `cache` 分层设计，L1/L2 独立模块

  ---

  ## 5. 配置说明

  ### 5.1 配置文件结构
  ```yaml
  # config.yaml
  server:
    host: 0.0.0.0
    port: 8080
    
  # PostgreSQL (持久化存储)
  database:
    host: postgres
    port: 5432
    user: llm_gateway
    password: ${DB_PASSWORD}
    name: llm_gateway
    
  # Redis (缓存/向量)
  redis:
    address: redis-stack:6379
    password: ""
    db: 0
    
  # Python Worker (独立服务)
  python_worker:
    address: python-worker:8081
    timeout: 5s
    
  providers:
    openai:
      api_key: ${OPENAI_API_KEY}
      base_url: https://api.openai.com/v1
    anthropic:
      api_key: ${ANTHROPIC_API_KEY}
    minimax:
      api_key: ${MINIMAX_API_KEY}
      base_url: https://api.minimax.chat/v1
  
  cache:
    enabled: true
    similarity_threshold: 0.95
    ttl: 604800  # 7 days
  
  ratelimit:
    global_qps: 10000
    burst: 500
    max_tokens: 128000
    # 按模型限流配置
    model_limits:
      gpt-4: 1000
      gpt-3.5-turbo: 3000
      claude-3-haiku: 2000
  
  models:
    - name: gpt-4
      provider: openai
      weight: 5
      fallback: gpt-3.5-turbo
      max_context: 8192
      tokenizer: tiktoken  # 指定 tokenizer 类型
    - name: gpt-3.5-turbo
      provider: openai
      weight: 3
      max_context: 16385
      tokenizer: tiktoken
    - name: claude-3-haiku
      provider: anthropic
      weight: 2
      max_context: 200000
      tokenizer: anthropic  # 使用估算方式
      
  # 配置管理 (轻量化方案)
  # 个人项目推荐: K8s ConfigMap + fsnotify 热更新
  config:
    source: k8s_configmap  # 挂载到 /etc/config/config.yaml
    hot_reload:
      enabled: true
      watch_path: /etc/config/
      # 使用 fsnotify 监听文件变更
      debounce: 500ms  # 防抖
      on_change:
        - reload_models      # 重载模型配置
        - reload_ratelimit   # 重载限流配置
        - reload_cache       # 重载缓存配置
    # 示例: K8s ConfigMap
    # kubectl create configmap llm-gateway-config --from-file=config.yaml
  
  monitoring:
    prometheus:
      enabled: true
      port: 9090
    jaeger:
      enabled: true
      endpoint: http://jaeger:14268/api/traces
  ```

  ### 5.2 环境变量
  ```bash
  # .env
  OPENAI_API_KEY=sk-xxxx
  ANTHROPIC_API_KEY=sk-ant-xxxx
  MINIMAX_API_KEY=xxxx
  
  REDIS_PASSWORD=
  JWT_SECRET=your-secret-key
  
  # K8s 部署时使用 Secret
  ```

  ---

  ## 6. 部署方案

  ### 6.1 Kubernetes 部署

  > ⚠️ **重要设计决策**: Python Worker 独立部署，非 Sidecar 模式
  > - 原因: Embedding 计算是 CPU 密集型，会抢占 Go Gateway 资源，影响 10k QPS 性能
  > - 架构: 两个独立 Deployment，通过 Service 内网调用

  #### 6.1.1 Go Gateway Deployment
  ```yaml
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: llm-gateway
  spec:
    replicas: 3
    selector:
      matchLabels:
        app: llm-gateway
    template:
      metadata:
        labels:
          app: llm-gateway
      spec:
        containers:
        - name: gateway
          image: llm-gateway:latest
          ports:
          - containerPort: 8080
          resources:
            requests:
              memory: "512Mi"
              cpu: "1000m"  # 提高 CPU 配额，保障 10k QPS
            limits:
              memory: "1Gi"
              cpu: "2000m"
          env:
          - name: REDIS_ADDRESS
            valueFrom:
              configMapKeyRef:
                name: llm-gateway-config
                key: redis.address
          - name: PYTHON_WORKER_URL
            value: "http://python-worker:8081"
  ```

  #### 6.1.2 Python Worker Deployment (独立)
  ```yaml
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: python-worker
  spec:
    replicas: 2  # 少于 Gateway，避免资源争抢
    selector:
      matchLabels:
        app: python-worker
    template:
      metadata:
        labels:
          app: python-worker
      spec:
        containers:
        - name: worker
          image: llm-gateway-python:latest
          ports:
          - containerPort: 8081
          resources:
            requests:
              memory: "2Gi"   # 大内存，用于向量模型加载
              cpu: "2000m"
            limits:
              memory: "4Gi"
              cpu: "4000m"   # 高 CPU 配额，用于 Embedding 计算
  ```

  #### 6.1.3 Service 配置
  ```yaml
  apiVersion: v1
  kind: Service
  metadata:
    name: llm-gateway
  spec:
    selector:
      app: llm-gateway
    ports:
    - port: 80
      targetPort: 8080
    type: ClusterIP
  
  ---
  apiVersion: v1
  kind: Service
  metadata:
    name: python-worker
  spec:
    selector:
      app: python-worker
    ports:
    - port: 8081
      targetPort: 8081
    # 仅集群内访问
  ```

  #### 6.1.4 HPA 配置

  ##### Gateway HPA
  ```yaml
  apiVersion: autoscaling/v2
  kind: HorizontalPodAutoscaler
  metadata:
    name: llm-gateway-hpa
  spec:
    scaleTargetRef:
      apiVersion: apps/v1
      kind: Deployment
      name: llm-gateway
    minReplicas: 3
    maxReplicas: 10
    metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
  ```

  ##### Python Worker HPA (计算密集型)
  ```yaml
  apiVersion: autoscaling/v2
  kind: HorizontalPodAutoscaler
  metadata:
    name: python-worker-hpa
  spec:
    scaleTargetRef:
      apiVersion: apps/v1
      kind: Deployment
      name: python-worker
    minReplicas: 2
    maxReplicas: 5
    metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 75  # CPU > 75% 自动扩容
    - type: Pods
      pods:
        metric:
          name: embedding_queue_length
        target:
          type: AverageValue
          averageValue: "10"  # 等待队列 > 10 时扩容
  ```

  ### 6.2 Docker Compose (开发环境)
  ```yaml
  version: '3.8'
  services:
    # Go Gateway - 主服务
    gateway:
      build: ./gateway
      ports:
        - "8080:8080"
      environment:
        - REDIS_ADDRESS=redis-stack:6379
        - PYTHON_WORKER_URL=http://python-worker:8081
        - DB_HOST=postgres
      depends_on:
        - redis-stack
        - python-worker
        - postgres
    
    # Python Worker - 独立部署 (非 Sidecar)
    python-worker:
      build: ./python-worker
      ports:
        - "8081:8081"
      environment:
        - REDIS_ADDRESS=redis-stack:6379
      deploy:
        resources:
          limits:
            cpus: '2'
            memory: 4G
    
    # PostgreSQL - 持久化存储
    postgres:
      image: postgres:15
      environment:
        POSTGRES_USER: llm_gateway
        POSTGRES_PASSWORD: dev_password
        POSTGRES_DB: llm_gateway
      ports:
        - "5432:5432"
      volumes:
        - postgres_data:/var/lib/postgresql/data
    
    # Redis Stack - 向量搜索 + 缓存
    redis-stack:
      image: redis/redis-stack:latest
      ports:
        - "6379:6379"
    
    # 监控组件
    prometheus:
      image: prom/prometheus:latest
      ports:
        - "9090:9090"
        
    grafana:
      image: grafana/grafana:latest
      ports:
        - "3000:3000"
  
  volumes:
    postgres_data:
  ```

  ---

  ## 7. 监控指标

  ### 7.1 核心指标

  | 指标名                            | 类型      | 描述         |
  | --------------------------------- | --------- | ------------ |
  | gateway_requests_total            | Counter   | 总请求数     |
  | gateway_requests_duration_seconds | Histogram | 请求延迟     |
  | gateway_cache_hits_total          | Counter   | 缓存命中数   |
  | gateway_cache_misses_total        | Counter   | 缓存未命中数 |
  | gateway_rate_limit_rejected_total | Counter   | 限流拒绝数   |
  | gateway_model_requests_total      | Counter   | 各模型请求数 |
  | gateway_model_errors_total        | Counter   | 各模型错误数 |
  | gateway_tokens_total              | Counter   | Token 消耗量 |

  ### 7.2 Grafana 面板

  #### 7.2.1 仪表盘指标
  - 实时 QPS
  - P50/P95/P99 延迟
  - 缓存命中率
  - 各模型调用占比
  - Token 消耗趋势
  - 成本估算
  - 错误率

  ---

  ## 8. API Key 管理

  ### 8.1 Key 格式
  ```
  sk-{随机字符串 32 位}
  ```

  ### 8.2 Key 属性
  ```json
  {
    "id": "key_xxxx",
    "key": "sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    "name": "Production Key",
    "rate_limit": 1000,
    "expires_at": "2025-12-31T23:59:59Z",
    "created_at": "2024-01-01T00:00:00Z",
    "is_active": true
  }
  ```

  ### 8.3 权限模型
  - **read**: 查询模型、统计
  - **write**: 创建 Key、修改配置
  - **admin**: 所有权限

  ---

  ## 9. 安全性

  ### 9.1 安全措施
  - [ ] API Key 加密存储
  - [ ] HTTPS/TLS 传输加密
  - [ ] 请求/响应日志脱敏
  - [ ] IP 白名单 (可选)
  - [ ] 请求体大小限制 (10MB)
  - [ ] 恶意请求检测

  ### 9.2 合规考虑
  - 不记录用户敏感数据到日志
  - API Key 不记录到日志
  - 定期轮换密钥

  ---

  ## 10. 成本优化

  ### 10.1 成本组成
  | 项目         | 预估成本 (月) | 说明                   |
  | ------------ | ------------- | ---------------------- |
  | LLM API 调用 | $500-2000     | 主要成本，视调用量而定 |
  | Redis Stack  | $50-100       | 4GB 内存配置           |
  | PostgreSQL   | $30-50        | 小型实例               |
  | K8s 集群     | $100-300      | 3-5 节点               |
  | 监控/日志    | $50-100       | ELK + Prometheus       |
  | **总计**     | **$730-2550** |                        |

  ### 10.2 LLM API 成本计算模型
  > ⚠️ **核心指标**: 精确追踪每个请求的 Token 消耗

  | 模型           | Input 价格      | Output 价格     | 计算公式                         |
  | -------------- | --------------- | --------------- | -------------------------------- |
  | GPT-4          | $0.03/1K tokens | $0.06/1K tokens | `input*0.03 + output*0.06`       |
  | GPT-3.5 Turbo  | $0.001/1K       | $0.002/1K       | `input*0.001 + output*0.002`     |
  | Claude-3 Haiku | $0.00025/1K     | $0.00125/1K     | `input*0.00025 + output*0.00125` |
  | MiniMax        | $0.001/1K       | $0.002/1K       | `input*0.001 + output*0.002`     |

  #### 10.2.1 成本统计 API
  ```bash
  # 按 API Key 统计成本
  GET /api/v1/stats/cost?key_id=xxx&start=2024-01&end=2024-02
  
  # 按模型统计成本
  GET /api/v1/stats/cost?group_by=model
  
  # 响应示例
  {
    "total_cost": 125.50,
    "by_model": {
      "gpt-4": 100.00,
      "gpt-3.5-turbo": 25.50
    },
    "by_key": {
      "key_xxx": 80.00,
      "key_yyy": 45.50
    },
    "currency": "USD"
  }
  ```

  ### 10.3 缓存收益计算
  > ⚠️ **缓存节省公式**: `节省成本 = 总请求 × 命中率 × 平均请求成本`

  | 场景       | 命中率 | 月请求量 | 单次成本 | 月节省   |
  | ---------- | ------ | -------- | -------- | -------- |
  | 无缓存     | 0%     | 100K     | $0.01    | $0       |
  | L1 缓存    | 60%    | 100K     | $0.01    | $600     |
  | L1+L2 缓存 | 80%    | 100K     | $0.01    | **$800** |

  > **预期收益**: 缓存命中后 **节省 40% API 成本**

  ### 10.4 优化策略
  1. **缓存命中**: 相似 Query 直接返回，不调用 LLM → 节省 40% 成本
  2. **模型选择**: 根据任务复杂度选择合适模型 (GPT-4 → GPT-3.5)
  3. **降级策略**: 故障时自动切换到便宜模型
  4. **限流保护**: 防止突发流量导致超额调用

  ---

  ## 11. 开发计划

  ### Phase 1: 基础框架 (1-2 周)
  - [ ] 项目初始化 (Go + Python)
  - [ ] 基础 HTTP 服务
  - [ ] 配置管理
  - [ ] 单模型调用 (OpenAI)

  ### Phase 2: 核心功能 (2-3 周)
  - [ ] 多模型支持
  - [ ] 负载均衡
  - [ ] 熔断降级
  - [ ] Token 限流

  ### Phase 3: 缓存系统 (2 周)
  - [ ] Redis Stack 集成
  - [ ] 向量 Embedding
  - [ ] 语义缓存逻辑

  ### Phase 4: 监控运维 (1-2 周)
  - [ ] Prometheus 集成
  - [ ] Grafana 面板
  - [ ] 日志收集

  ### Phase 5: 管理后台 (2 周)
  - [ ] Admin API
  - [ ] 前端 Dashboard
  - [ ] Key 管理

  ---

  ## 12. 验收标准

  ### 12.1 功能验收
  - [ ] OpenAI 兼容 API 调用成功
  - [ ] Claude/ Minimax 模型调用成功
  - [ ] 缓存命中时延迟 < 50ms
  - [ ] 限流正确拦截超出 QPS 的请求
  - [ ] 故障时自动降级

  ### 12.2 性能验收
  - [ ] 单实例支持 10k QPS
  - [ ] P99 延迟 < 500ms
  - [ ] 缓存命中率 > 30% (典型场景)

  ### 12.3 运维验收
  - [ ] K8s 部署成功
  - [ ] 监控数据正确采集
  - [ ] 日志正确收集

  ### 12.4 性能测试方案
  > ⚠️ **让 10k QPS 可信**: 具体可落地的压测方法

  | 测试场景    | 工具     | 目标指标   | 说明                        |
  | ----------- | -------- | ---------- | --------------------------- |
  | L1 缓存命中 | k6 / wrk | 50k+ QPS   | 验证极限吞吐 (Hash 查询)    |
  | L2 缓存命中 | k6 / wrk | 5k-10k QPS | 验证 Embedding 链路         |
  | 缓存全穿透  | k6 / wrk | 10k QPS    | 验证网关转发能力 (Mock LLM) |
  | 混合场景    | k6 / wrk | 10k QPS    | 80% L1 + 10% L2 + 10% Miss  |

  #### 12.4.1 压测工具选择
  ```bash
  # 方案 A: k6 (推荐)
  # - JavaScript 编写脚本
  # - 支持 Grafana 集成
  k6 run --vus 100 --duration 30s script.js
  
  # 方案 B: wrk
  # - 轻量级命令行工具
  wrk -t12 -c400 -d30s http://localhost:8080/v1/chat/completions
  ```

  #### 12.4.2 本地压测环境 (Docker Compose)
  ```yaml
  # 分配足够资源
  services:
    gateway:
      deploy:
        resources:
          limits:
            cpus: '4'
            memory: 8G
  ```

  #### 12.4.3 Mock LLM (避免 API 成本)
  ```go
  // Go Gateway 测试模式
  func mockLLMHandler() http.Handler {
      return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          time.Sleep(50 * time.Millisecond) // 模拟 LLM 延迟
          w.Write([]byte(`{"choices": [{"message": {"content": "test"}}]}`))
      })
  }
  ```

  > **面试话术**: "压测使用 k6 工具，分为 4 个场景：L1 极限吞吐、L2 语义缓存、网关转发能力、混合场景。测试结果会实时展示在 Grafana 面板上，确保数据可信。"

  ---

  ### 12.5 扩展性设计

  #### 12.5.1 新 LLM Provider 接入
  > ⚠️ **适配器模式**: 5 分钟快速接入新模型

  ```go
  // 1. 实现 Provider 接口
  type Provider interface {
      Chat(ctx context.Context, req *Request) (*Response, error)
      Embedding(ctx context.Context, text string) ([]float64, error)
      GetModelInfo() *ModelInfo
  }
  
  // 2. 注册到 Router
  func init() {
      RegisterProvider("new-model", &NewModelProvider{})
  }
  
  // 3. 配置启用
  # config.yaml
  models:
    - name: new-model
      provider: new-model  # 自动匹配
      weight: 1
  ```

  #### 12.5.2 多租户支持 (预留)
  | 字段           | 说明               |
  | -------------- | ------------------ |
  | tenant_id      | 租户标识           |
  | quota_monthly  | 月度配额           |
  | rate_limit     | 租户级限流         |
  | models_allowed | 允许使用的模型列表 |

  ```sql
  -- 租户表
  CREATE TABLE tenants (
      id UUID PRIMARY KEY,
      name VARCHAR(255),
      quota_monthly DECIMAL(10, 2),
      is_active BOOLEAN DEFAULT true,
      created_at TIMESTAMP DEFAULT NOW()
  );
  
  -- 租户配额表
  CREATE TABLE tenant_quotas (
      tenant_id UUID REFERENCES tenants(id),
      month VARCHAR(7),  -- 2024-01
      tokens_used BIGINT DEFAULT 0,
      cost_used DECIMAL(10, 2) DEFAULT 0,
      PRIMARY KEY (tenant_id, month)
  );
  ```

  #### 12.5.3 插件化设计 (预留)
  - **Filter 插件**: 请求/响应拦截 (日志脱敏、敏感词过滤)
  - **Middleware 插件**: 自定义认证、限流策略
  - 实现接口: `Plugin interface { Name() string; Init() error }`

  > **面试话术**: "网关采用适配器模式，新模型接入只需实现接口并注册，5 分钟完成。多租户和插件化是预留设计，后续业务复杂时可快速扩展。"

  ---

  ## 13. 附录

  ### 13.1 参考资料
  - [OpenAI API Docs](https://platform.openai.com/docs)
  - [Redis Stack向量搜索](https://redis.io/docs/stack/search/)
  - [TikToken](https://github.com/openai/tiktoken)
  - [Kubernetes Documentation](https://kubernetes.io/docs/)

  ### 13.2 术语表
  | 术语             | 解释                 |
  | ---------------- | -------------------- |
  | QPS              | 每秒查询数           |
  | P99              | 99% 分位延迟         |
  | Token            | LLM 输入/输出单位    |
  | Vector Embedding | 文本向量表示         |
  | 语义缓存         | 基于向量相似度的缓存 |

  ---

  *文档版本: v1.1*  
  *最后更新: 2026-02-15*
