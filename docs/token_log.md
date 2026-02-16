# Token Log

## 用户
张世斌 | 18622412361@163.com | GitHub: Oxidaner

## 背景
- 硕士: 香港理工大学 2026.09-2028.06 (已录取)
- 本科: 天津理工大学 2020.09-2024.07
- 求职: Golang后端开发 / AI应用开发工程师(实习)

## 技能 (实际)
- Golang: 精通 ✓
- Python: **零基础** ← 重要
- Redis: 熟悉基础，**向量搜索需学习** ← 重要
- MySQL/Docker: 熟悉 ✓

## 项目: High-Performance LLM Gateway
技术栈: Go + Python + Redis Stack + PostgreSQL + K8s

### 核心功能
- L1精确缓存: Redis Hash (SHA256 prompt)，< 1ms
- L2语义缓存: Redis Vector (Embedding相似度>0.95)，10-50ms
- Token限流: 令牌桶 + TikToken Go，10k QPS
- 多模型路由: 加权轮询 + 熔断降级
- 认证: API Key (PostgreSQL)

## 开发阶段
Phase 0: 基础知识学习 ← **当前**

## 文档状态
- SPEC.md (v1.1) ✓
- Todo.md (v1.3) ✓ - 已细化120+Task，含学习网址
- token_log.md ✓

## 常用链接 (速查)
- Python: https://www.runoob.com/python3/python3-tutorial.html
- FastAPI: https://fastapi.tiangolo.com/zh/tutorial/
- Redis Vector: https://redis.io/docs/stack/search/vector-similarity/
- Gin: https://gin-gonic.com/zh-cn/docs/quickstart/
- K8s: https://kubernetes.io/zh-cn/docs/tutorials/

## 编码状态
进行中 - Go框架搭建完成 (Phase 0.4)

## 已完成功能 (2026-02-16)
- Go项目初始化 + Gin框架
- HTTP服务骨架 + 健康检查
- 配置文件加载 (config.yaml)
- Zap日志库配置
- /v1/chat/completions 接口 (框架)
- /v1/embeddings 接口 (框架)
- /v1/models 接口
- Admin API: Key CRUD
- API Key 认证中间件 (框架)
- Token Bucket 限流 (框架)
- Redis 客户端 (框架)
- PostgreSQL 客户端 (框架)
- L1 缓存读写 (框架)

## 对话历史
2026-02-15: 完成SPEC.md、Todo.md(细化版+网址)、token_log.md
2026-02-16: 完成Go框架搭建(Task 0.4)
2026-02-16: 完成Zap日志库(Task 0.4.4)

---
v1.4 | 2026-02-16
