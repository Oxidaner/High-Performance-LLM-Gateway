package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Logger        LoggerConfig        `yaml:"logger"`
	Database      DatabaseConfig      `yaml:"database"`
	Redis         RedisConfig         `yaml:"redis"`
	PythonWorker  PythonWorkerConfig  `yaml:"python_worker"`
	Providers     ProvidersConfig    `yaml:"providers"`
	Cache         CacheConfig        `yaml:"cache"`
	RateLimit     RateLimitConfig    `yaml:"ratelimit"`
	Models        []ModelConfig      `yaml:"models"`
	Monitoring    MonitoringConfig   `yaml:"monitoring"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"` // debug, release
}

type LoggerConfig struct {
	Level      string `yaml:"level"`      // debug, info, warn, error
	Format     string `yaml:"format"`     // json, console
	OutputPath string `yaml:"output_path"` // stdout, stderr, or file path
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"name"`
}

type RedisConfig struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type PythonWorkerConfig struct {
	Address string        `yaml:"address"`
	Timeout time.Duration `yaml:"timeout"`
}

type ProvidersConfig struct {
	OpenAI   ProviderConfig `yaml:"openai"`
	Anthropic ProviderConfig `yaml:"anthropic"`
	MiniMax  ProviderConfig `yaml:"minimax"`
}

type ProviderConfig struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
}

type CacheConfig struct {
	Enabled             bool    `yaml:"enabled"`
	SimilarityThreshold float64 `yaml:"similarity_threshold"`
	TTL                 int     `yaml:"ttl"` // seconds
	L1TTL               int     `yaml:"l1_ttl"`
	MaxCacheSize        int     `yaml:"max_cache_size"`
}

type RateLimitConfig struct {
	GlobalQPS     int               `yaml:"global_qps"`
	Burst         int               `yaml:"burst"`
	MaxTokens     int               `yaml:"max_tokens"`
	ModelLimits   map[string]int   `yaml:"model_limits"`
}

type ModelConfig struct {
	Name        string `yaml:"name"`
	Provider    string `yaml:"provider"`
	Weight      int    `yaml:"weight"`
	Fallback    string `yaml:"fallback"`
	MaxContext  int    `yaml:"max_context"`
	Tokenizer   string `yaml:"tokenizer"`
	IsActive    bool   `yaml:"is_active"`
}

type MonitoringConfig struct {
	Prometheus PrometheusConfig `yaml:"prometheus"`
	Jaeger     JaegerConfig    `yaml:"jaeger"`
}

type PrometheusConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
}

type JaegerConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Endpoint string `yaml:"endpoint"`
}

// Load reads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides
	cfg.applyEnvOverrides()

	return &cfg, nil
}

func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		c.Database.Password = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		c.Providers.OpenAI.APIKey = v
	}
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		c.Providers.Anthropic.APIKey = v
	}
	if v := os.Getenv("MINIMAX_API_KEY"); v != "" {
		c.Providers.MiniMax.APIKey = v
	}
	if v := os.Getenv("REDIS_ADDRESS"); v != "" {
		c.Redis.Address = v
	}
	if v := os.Getenv("PYTHON_WORKER_URL"); v != "" {
		c.PythonWorker.Address = v
	}
}
