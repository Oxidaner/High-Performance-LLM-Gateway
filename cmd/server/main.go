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

	"llm-gateway/internal/config"
	"llm-gateway/internal/handler"
	"llm-gateway/internal/middleware"
	"llm-gateway/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

var mode = flag.String("mode", "release", "run mode: debug or release")

func main() {
	flag.Parse()

	// Load .env file if exists
	_ = godotenv.Load()

	// Load configuration
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Override mode from command line
	// sample:
	//	# 默认正式环境        # 开发模式                         # 正式环境
	//  ./llm-gateway        ./llm-gateway -mode=debug         ./llm-gateway -mode=release
	if *mode == "debug" || *mode == "release" {
		cfg.Server.Mode = *mode
	}

	// Initialize logger (zap)
	if err := middleware.Init(middleware.Config{
		Level:      cfg.Logger.Level,
		Format:     cfg.Logger.Format,
		OutputPath: cfg.Logger.OutputPath,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1) //退出程序
	}
	defer middleware.Sync() // 确保在程序退出时刷新日志

	middleware.Info("Starting LLM Gateway",
		middleware.String("mode", cfg.Server.Mode),
		middleware.String("addr", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)),
	)

	// Initialize storage clients
	// 初始化存储客户端 - debug模式下允许连接失败后继续运行
	var redisClient *storage.RedisClient
	redisClient, err = storage.NewRedis(cfg.Redis)
	if err != nil {
		if cfg.Server.Mode == "debug" {
			middleware.Warn("Failed to connect to Redis, continuing without Redis",
				middleware.Err(err),
			)
			redisClient = nil
		} else {
			middleware.Fatal("Failed to connect to Redis in release mode, exiting",
				middleware.Err(err),
			)
			os.Exit(1) // 退出程序
		}
	} else {
		defer redisClient.Close()
	}

	var postgresClient *storage.PostgresClient
	postgresClient, err = storage.NewPostgres(cfg.Database)
	if err != nil {
		if cfg.Server.Mode == "debug" {
			middleware.Warn("Failed to connect to PostgreSQL, continuing without DB",
				middleware.Err(err),
			)
			postgresClient = nil
		} else {
			middleware.Fatal("Failed to connect to PostgreSQL in release mode, exiting",
				middleware.Err(err),
			)
			os.Exit(1)
		}
	} else {
		defer postgresClient.Close()
	}

	// Set Gin mode
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	/**
		func New(opts ...OptionFunc) *Engine {
		debugPrintWARNINGNew()
		engine := &Engine{
			RouterGroup: RouterGroup{
				Handlers: nil,
				basePath: "/",
				root:     true,
			},
			FuncMap:                template.FuncMap{}, // 自定义模板函数 一个map，用于在模板中调用自定义函数
			RedirectTrailingSlash:  true, // 重定向 trailing slash 到没有 trailing slash 的 URL
			RedirectFixedPath:      false, // 重定向固定路径到没有固定路径的 URL
			HandleMethodNotAllowed: false, // 是否处理 HTTP 方法不允许的情况
			ForwardedByClientIP:    true,
			RemoteIPHeaders:        []string{"X-Forwarded-For", "X-Real-IP"},
			TrustedPlatform:        defaultPlatform,
			UseRawPath:             false,
			RemoveExtraSlash:       false,
			UnescapePathValues:     true,
			MaxMultipartMemory:     defaultMultipartMemory,
			trees:                  make(methodTrees, 0, 9),

			delims:                 render.Delims{Left: "{{", Right: "}}"},
			secureJSONPrefix:       "while(1);",
			trustedProxies:         []string{"0.0.0.0/0", "::/0"},
			trustedCIDRs:           defaultTrustedCIDRs,
		}
		engine.engine = engine
		engine.pool.New = func() any {
			return engine.allocateContext(engine.maxParams)
		}
		return engine.With(opts...)
	}
	*/

	// Create router
	router := gin.Default()

	// Apply global middleware
	router.Use(middleware.Recovery()) // 恢复中间件，用于处理panic
	router.Use(middleware.CORS())     // CORS 中间件，用于处理跨域请求

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
		middleware.Info("Server listening", middleware.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			middleware.Fatal("Server failed to start", middleware.Err(err))
		}
	}()

	// Wait for interrupt signal
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
