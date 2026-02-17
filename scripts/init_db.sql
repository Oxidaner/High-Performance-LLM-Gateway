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