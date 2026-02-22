# Token Log

## 用户
张世斌 | 18622412361@163.com | GitHub: Oxidaner

## 背景
- 硕士: 香港理工大学 2026.09-2028.06 (已录取)
- 本科: 天津理工大学 2020.09-2024.07
- 求职: Golang后端开发 / AI应用开发工程师(实习)

## 技能 (实际)
- Golang: 精通 ✓
- Python: 基础 ← 学习中
- Redis: 熟悉，**向量搜索已学习** ← 重要
- MySQL/Docker: 熟悉 ✓

## 项目: High-Performance LLM Gateway
技术栈: Go + Python + Redis Stack + PostgreSQL + K8s

### 核心功能
- LLM网关: OpenAI/Claude/MiniMax 统一接入
- L1精确缓存: Redis Hash (SHA256 prompt)，< 1ms
- L2语义缓存: Redis Vector (Embedding相似度>0.95)，10-50ms
- Token限流: 令牌桶 + TikToken Go，10k QPS
- 多模型路由: 加权轮询 + 熔断降级
- **AI Agent**: ReAct/CoT 推理 + 工具调用 + 混合模式 (默认技能集内置 + 动态发现)
- **RAG**: 文档上传 → 向量检索 → LLM生成
- **智能重试**: 指数退避 + 可重试错误码
- **Prompt优化**: 系统提示词缓存 + 历史消息压缩
- **调用链观测**: OpenTelemetry/Jaeger
- 认证: API Key (PostgreSQL)

## 开发阶段
Phase 6: AI Agent + RAG 开发中

## 文档状态
- SPEC.md (v1.4) ✓ - 含 Agent/RAG/重试/追踪
- Todo.md (v1.8) ✓ - 含 Phase 6 任务
- CLAUDE.md ✓ - 项目指南
- token_log.md ✓

## 常用链接 (速查)
- Python: https://www.runoob.com/python3/python3-tutorial.html
- FastAPI: https://fastapi.tiangolo.com/zh/tutorial/
- Redis Vector: https://redis.io/docs/stack/search/vector-similarity/
- Gin: https://gin-gonic.com/zh-cn/docs/quickstart/
- K8s: https://kubernetes.io/zh-cn/docs/tutorials/

## 编码状态
进行中 - M2 OpenAI API 调用完成 ✓

## 当前代码状态 (2026-02-21)

### 已实现
- Go项目初始化 + Gin框架
- HTTP服务骨架 + 健康检查
- 配置文件加载 (config.yaml)
- Zap日志库配置
- /v1/chat/completions 接口 (直接HTTP调用)
- /v1/embeddings 接口 (转发Python Worker)
- /v1/models 接口
- Admin API: Key CRUD
- API Key 认证中间件 (框架)
- Token Bucket 限流 (框架)
- Redis 客户端
- PostgreSQL 客户端
- L1 缓存读写

### 待实现 (Phase 6)
- AI Agent (ReAct/CoT 推理引擎)
- RAG (文档上传、向量检索、知识库)
- 智能重试 (指数退避)
- Prompt 优化 (缓存、压缩)
- 调用链观测 (OpenTelemetry/Jaeger)
- Python Worker 服务
- L2 语义缓存
- TikToken 精确计算
- 多模型负载均衡/熔断
- K8s 部署配置

## GitHub
https://github.com/Oxidaner/High-Performance-LLM-Gateway

## 对话历史
2026-02-15: 完成SPEC.md、Todo.md(细化版+网址)、token_log.md
2026-02-16: 完成Go框架搭建(Task 0.4)
2026-02-16: 完成Zap日志库(Task 0.4.4)
2026-02-17: 更新所有docs文档(CLAUDE.md, SPEC.md, Todo.md, token_log.md)
2026-02-17: 代码扫描更新Todo状态
2026-02-21: 新增AI Agent、RAG、智能重试、Prompt优化、调用链观测功能规格

---
v1.7 | 2026-02-21
