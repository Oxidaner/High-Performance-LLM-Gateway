package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"llm-gateway/internal/config"
	"llm-gateway/internal/handler"
	"llm-gateway/internal/logger"
	"llm-gateway/internal/middleware"
	"llm-gateway/internal/storage"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger (zap)
	if err := logger.Init(logger.Config{
		Level:      cfg.Logger.Level,
		Format:     cfg.Logger.Format,
		OutputPath: cfg.Logger.OutputPath,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting LLM Gateway",
		logger.String("mode", cfg.Server.Mode),
		logger.String("addr", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)),
	)

	// Initialize storage clients (optional - continue if failed in dev mode)
	var redisClient *storage.RedisClient
	redisClient, err = storage.NewRedis(cfg.Redis)
	if err != nil {
		logger.Warn("Failed to connect to Redis, continuing without Redis",
			logger.Err(err),
		)
		redisClient = nil
	} else {
		defer redisClient.Close()
	}

	var postgresClient *storage.PostgresClient
	postgresClient, err = storage.NewPostgres(cfg.Database)
	if err != nil {
		logger.Warn("Failed to connect to PostgreSQL, continuing without DB",
			logger.Err(err),
		)
		postgresClient = nil
	} else {
		defer postgresClient.Close()
	}

	// Set Gin mode
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := gin.Default()

	// Apply global middleware
	router.Use(middleware.Recovery())
	router.Use(middleware.CORS())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// API v1 routes
	v1 := router.Group("/v1")
	{
		// Chat completions
		v1.POST("/chat/completions",
			middleware.APIKeyAuth(redisClient),
			middleware.RateLimit(cfg.RateLimit),
			handler.ChatCompletion(cfg, redisClient),
		)

		// Embeddings
		v1.POST("/embeddings",
			middleware.APIKeyAuth(redisClient),
			handler.EmbeddingHandler(cfg),
		)

		// Models list
		v1.GET("/models", handler.ListModels(cfg))
	}

	// Admin API routes
	admin := router.Group("/api/v1")
	{
		admin.POST("/keys", handler.CreateAPIKey(postgresClient, redisClient))
		admin.GET("/keys", handler.ListAPIKeys(postgresClient))
		admin.DELETE("/keys/:id", handler.DeleteAPIKey(postgresClient, redisClient))
		admin.GET("/stats", handler.GetStats(postgresClient))
	}

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		logger.Info("Server listening", logger.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed to start", logger.Err(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", logger.Err(err))
	}

	logger.Info("Server exited")
}
