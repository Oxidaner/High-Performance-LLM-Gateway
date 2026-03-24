package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"llm-gateway/internal/config"
	"llm-gateway/internal/handler"
	"llm-gateway/internal/middleware"
	gatewayservice "llm-gateway/internal/service"
	"llm-gateway/internal/service/cache"
	"llm-gateway/internal/service/embeddingworker"
	"llm-gateway/internal/service/provider"
	"llm-gateway/internal/storage"
	"llm-gateway/internal/telemetry"
)

var mode = flag.String("mode", "release", "run mode: debug or release")

func main() {
	flag.Parse()

	_ = godotenv.Load()

	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if *mode == "debug" || *mode == "release" {
		cfg.Server.Mode = *mode
	}
	if cfg.Workflow.ReplayLogPath != "" {
		gatewayservice.ConfigureDefaultWorkflowTracer(cfg.Workflow.ReplayLogPath)
	}

	if err := middleware.Init(middleware.Config{
		Level:      cfg.Logger.Level,
		Format:     cfg.Logger.Format,
		OutputPath: cfg.Logger.OutputPath,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer middleware.Sync()

	shutdownTelemetry, err := telemetry.Init(cfg.Monitoring)
	if err != nil {
		if cfg.Server.Mode == "debug" {
			middleware.Warn("Failed to initialize OpenTelemetry, continuing without tracing", middleware.Err(err))
			shutdownTelemetry = func(context.Context) error { return nil }
		} else {
			middleware.Fatal("Failed to initialize OpenTelemetry in release mode, exiting", middleware.Err(err))
			os.Exit(1)
		}
	}
	defer shutdownTelemetry(context.Background())

	middleware.Info("Starting LLM Gateway",
		middleware.String("mode", cfg.Server.Mode),
		middleware.String("addr", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)),
	)

	redisClient, postgresClient := initStorage(cfg)
	if redisClient != nil {
		defer redisClient.Close()
	}
	if postgresClient != nil {
		defer postgresClient.Close()
	}

	l1Cache, l2Cache := initCaches(cfg, redisClient)

	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	providerRegistry := provider.NewRegistry(cfg)
	workerClient := embeddingworker.NewClient(embeddingworker.Config{
		Address:      cfg.PythonWorker.Address,
		Timeout:      cfg.PythonWorker.Timeout,
		RetryMax:     cfg.PythonWorker.RetryMax,
		RetryBackoff: cfg.PythonWorker.RetryBackoff,
		HealthTTL:    cfg.PythonWorker.HealthTTL,
	})

	router := gin.Default()
	router.Use(middleware.Recovery())
	router.Use(middleware.CORS())
	router.Use(middleware.Tracing())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	v1 := router.Group("/v1")
	{
		v1.POST("/chat/completions",
			middleware.APIKeyAuth(redisClient),
			middleware.RateLimit(cfg.RateLimit),
			handler.ChatCompletion(cfg, providerRegistry, workerClient, postgresClient, l1Cache, l2Cache),
		)

		v1.POST("/embeddings",
			middleware.APIKeyAuth(redisClient),
			handler.EmbeddingHandler(cfg, workerClient, postgresClient),
		)

		v1.GET("/models", handler.ListModels(cfg))
	}

	admin := router.Group("/api/v1")
	{
		admin.POST("/keys", handler.CreateAPIKey(postgresClient, redisClient))
		admin.GET("/keys", handler.ListAPIKeys(postgresClient))
		admin.DELETE("/keys/:id", handler.DeleteAPIKey(postgresClient, redisClient))
		admin.GET("/stats", handler.GetStats(postgresClient))
		admin.GET("/workflows/:session_id/summary", handler.GetWorkflowSummary())
		admin.GET("/workflows/summaries", handler.ListWorkflowSummaries())
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: router,
	}

	go func() {
		middleware.Info("Server listening", middleware.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			middleware.Fatal("Server failed to start", middleware.Err(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	middleware.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		middleware.Fatal("Server forced to shutdown", middleware.Err(err))
	}

	middleware.Info("Server exited")
}

func initStorage(cfg *config.Config) (*storage.RedisClient, *storage.PostgresClient) {
	var redisClient *storage.RedisClient
	redisClient, redisErr := storage.NewRedis(cfg.Redis)
	if redisErr != nil {
		if cfg.Server.Mode == "debug" {
			middleware.Warn("Failed to connect to Redis, continuing without Redis", middleware.Err(redisErr))
		} else {
			middleware.Fatal("Failed to connect to Redis in release mode, exiting", middleware.Err(redisErr))
			os.Exit(1)
		}
	}

	var postgresClient *storage.PostgresClient
	postgresClient, pgErr := storage.NewPostgres(cfg.Database)
	if pgErr != nil {
		if cfg.Server.Mode == "debug" {
			middleware.Warn("Failed to connect to PostgreSQL, continuing without DB", middleware.Err(pgErr))
		} else {
			middleware.Fatal("Failed to connect to PostgreSQL in release mode, exiting", middleware.Err(pgErr))
			os.Exit(1)
		}
	} else {
		if schemaErr := postgresClient.EnsureRequestLogSchema(context.Background()); schemaErr != nil {
			if cfg.Server.Mode == "debug" {
				middleware.Warn("Failed to ensure request log schema, continuing without durable request logs", middleware.Err(schemaErr))
			} else {
				middleware.Fatal("Failed to ensure request log schema in release mode, exiting", middleware.Err(schemaErr))
				os.Exit(1)
			}
		}
	}

	return redisClient, postgresClient
}

func initCaches(cfg *config.Config, redisClient *storage.RedisClient) (*cache.L1Cache, *cache.L2Cache) {
	if redisClient == nil || !cfg.Cache.Enabled {
		return nil, nil
	}

	redis := redisClient.Client()
	l1Cache := cache.NewL1Cache(redis, cache.L1CacheConfig{
		Enabled:   cfg.Cache.Enabled,
		TTL:       cfg.Cache.L1TTL,
		MaxSize:   int64(cfg.Cache.MaxCacheSize),
		KeyPrefix: "cache:l1",
	})
	l2Cache := cache.NewL2Cache(redis, cache.L2CacheConfig{
		Enabled:             cfg.Cache.Enabled,
		SimilarityThreshold: cfg.Cache.SimilarityThreshold,
		TTL:                 cfg.Cache.TTL,
		MaxSize:             int64(cfg.Cache.MaxCacheSize),
		KeyPrefix:           "cache:l2",
		VectorDim:           384,
	})

	middleware.Info("Cache initialized",
		middleware.String("l1_ttl", fmt.Sprintf("%ds", cfg.Cache.L1TTL)),
		middleware.String("l2_similarity", fmt.Sprintf("%.2f", cfg.Cache.SimilarityThreshold)),
	)

	return l1Cache, l2Cache
}
