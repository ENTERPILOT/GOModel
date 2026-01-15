// Package main is the entry point for the LLM gateway server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"gomodel/config"
	"gomodel/internal/auditlog"
	"gomodel/internal/cache"
	"gomodel/internal/observability"
	"gomodel/internal/providers"
	"gomodel/internal/storage"

	// Import provider packages to trigger their init() registration
	_ "gomodel/internal/providers/anthropic"
	_ "gomodel/internal/providers/gemini"
	_ "gomodel/internal/providers/groq"
	_ "gomodel/internal/providers/openai"
	_ "gomodel/internal/providers/xai"
	"gomodel/internal/server"
	"gomodel/internal/version"
)

// getCacheDir returns the directory for cache files.
// Uses $GOMODEL_CACHE_DIR if set, otherwise ./.cache (working directory)
func getCacheDir() string {
	if cacheDir := os.Getenv("GOMODEL_CACHE_DIR"); cacheDir != "" {
		return cacheDir
	}
	return ".cache"
}

// initCache initializes the appropriate cache backend based on configuration.
// Returns a local file cache by default, or Redis if configured.
func initCache(cfg *config.Config) (cache.Cache, error) {
	cacheType := cfg.Cache.Type
	if cacheType == "" {
		cacheType = "local"
	}

	switch cacheType {
	case "redis":
		ttl := time.Duration(cfg.Cache.Redis.TTL) * time.Second
		if ttl == 0 {
			ttl = cache.DefaultRedisTTL
		}

		redisCfg := cache.RedisConfig{
			URL: cfg.Cache.Redis.URL,
			Key: cfg.Cache.Redis.Key,
			TTL: ttl,
		}

		redisCache, err := cache.NewRedisCache(redisCfg)
		if err != nil {
			return nil, err
		}

		slog.Info("using redis cache", "url", cfg.Cache.Redis.URL, "key", cfg.Cache.Redis.Key)
		return redisCache, nil

	default: // "local" or any other value defaults to local
		cacheFile := filepath.Join(getCacheDir(), "models.json")
		slog.Info("using local file cache", "path", cacheFile)
		return cache.NewLocalCache(cacheFile), nil
	}
}

func main() {
	// Add a version flag check
	versionFlag := flag.Bool("version", false, "Print version information")
	flag.Parse()

	if *versionFlag {
		fmt.Println(version.Info())
		os.Exit(0)
	}

	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Log the version immediately on startup
	slog.Info("starting gomodel",
		"version", version.Version,
		"commit", version.Commit,
		"build_date", version.Date,
	)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Validate that at least one provider is configured
	if len(cfg.Providers) == 0 {
		slog.Error("at least one provider must be configured")
		os.Exit(1)
	}

	// Setup observability hooks for metrics collection (if enabled)
	// This must be done BEFORE creating providers so they can use the hooks
	if cfg.Metrics.Enabled {
		metricsHooks := observability.NewPrometheusHooks()
		providers.SetGlobalHooks(metricsHooks)
		slog.Info("prometheus metrics enabled", "endpoint", cfg.Metrics.Endpoint)
	} else {
		slog.Info("prometheus metrics disabled")
	}

	// Initialize cache backend based on configuration
	modelCache, err := initCache(cfg)
	if err != nil {
		slog.Error("failed to initialize cache", "error", err)
		os.Exit(1)
	}
	defer modelCache.Close()

	// Create model registry with cache for instant startup
	registry := providers.NewModelRegistry()
	registry.SetCache(modelCache)

	// Sort provider names for deterministic initialization order
	providerNames := make([]string, 0, len(cfg.Providers))
	for name := range cfg.Providers {
		providerNames = append(providerNames, name)
	}
	sort.Strings(providerNames)

	// Create providers dynamically using the factory and register them
	var initializedCount int
	for _, name := range providerNames {
		pCfg := cfg.Providers[name]
		p, err := providers.Create(pCfg)
		if err != nil {
			slog.Error("failed to initialize provider", "name", name, "type", pCfg.Type, "error", err)
			continue
		}
		// Register with type for cache persistence
		registry.RegisterProviderWithType(p, pCfg.Type)
		initializedCount++
		slog.Info("provider initialized", "name", name, "type", pCfg.Type)
	}

	// Validate that at least one provider was successfully initialized
	if initializedCount == 0 {
		slog.Error("no providers were successfully initialized")
		os.Exit(1)
	}

	// Non-blocking initialization: load from cache, then refresh in background
	// This allows the server to start serving traffic immediately using cached data
	slog.Info("starting non-blocking model registry initialization...")
	registry.InitializeAsync(context.Background())

	slog.Info("model registry configured",
		"cached_models", registry.ModelCount(),
		"providers", registry.ProviderCount(),
	)

	// Start background refresh of model registry (every 5 minutes)
	// This keeps the model list up-to-date as providers add/remove models
	stopRefresh := registry.StartBackgroundRefresh(5 * time.Minute)
	defer stopRefresh()

	// Create provider router
	router, err := providers.NewRouter(registry)
	if err != nil {
		slog.Error("failed to create router", "error", err)
		os.Exit(1)
	}

	// Security check: warn if no master key is configured
	if cfg.Server.MasterKey == "" {
		slog.Warn("SECURITY WARNING: GOMODEL_MASTER_KEY not set - server running in UNSAFE MODE",
			"security_risk", "unauthenticated access allowed",
			"recommendation", "set GOMODEL_MASTER_KEY environment variable to secure this gateway")
	} else {
		slog.Info("authentication enabled", "mode", "master_key")
	}

	// Initialize audit logging if enabled
	var auditLogger auditlog.LoggerInterface = &auditlog.NoopLogger{}
	if cfg.Logging.Enabled {
		var auditStore storage.Storage
		auditLogger, auditStore, err = initAuditLogger(cfg)
		if err != nil {
			slog.Error("failed to initialize audit logging", "error", err)
			os.Exit(1)
		}
		defer auditLogger.Close()
		defer auditStore.Close()
		slog.Info("audit logging enabled",
			"storage_type", cfg.Logging.StorageType,
			"log_bodies", cfg.Logging.LogBodies,
			"log_headers", cfg.Logging.LogHeaders,
			"retention_days", cfg.Logging.RetentionDays,
		)
	} else {
		slog.Info("audit logging disabled")
	}

	// Create and start server
	serverCfg := &server.Config{
		MasterKey:                cfg.Server.MasterKey,
		MetricsEnabled:           cfg.Metrics.Enabled,
		MetricsEndpoint:          cfg.Metrics.Endpoint,
		BodySizeLimit:            cfg.Server.BodySizeLimit,
		AuditLogger:              auditLogger,
		LogOnlyModelInteractions: cfg.Logging.OnlyModelInteractions,
	}
	srv := server.New(router, serverCfg)

	// Handle graceful shutdown
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		slog.Info("shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
	}()

	addr := ":" + cfg.Server.Port
	slog.Info("starting server", "address", addr)

	if err := srv.Start(addr); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			slog.Info("server stopped gracefully")
		} else {
			slog.Error("server failed to start", "error", err)
			os.Exit(1)
		}
	}
}

// initAuditLogger initializes the audit logger with the appropriate storage backend.
// Returns the logger, the underlying storage (for cleanup), and any error.
func initAuditLogger(cfg *config.Config) (auditlog.LoggerInterface, storage.Storage, error) {
	// Create storage configuration using LOGGING_STORAGE_TYPE to select the backend
	// but using the shared storage connection settings (SQLITE_PATH, POSTGRES_URL, etc.)
	storageCfg := storage.Config{
		Type: cfg.Logging.StorageType,
		SQLite: storage.SQLiteConfig{
			Path: cfg.Storage.SQLite.Path,
		},
		PostgreSQL: storage.PostgreSQLConfig{
			URL:      cfg.Storage.PostgreSQL.URL,
			MaxConns: cfg.Storage.PostgreSQL.MaxConns,
		},
		MongoDB: storage.MongoDBConfig{
			URL:      cfg.Storage.MongoDB.URL,
			Database: cfg.Storage.MongoDB.Database,
		},
	}

	// Apply defaults
	if storageCfg.Type == "" {
		storageCfg.Type = storage.TypeSQLite
	}
	if storageCfg.SQLite.Path == "" {
		storageCfg.SQLite.Path = ".cache/gomodel.db"
	}
	if storageCfg.MongoDB.Database == "" {
		storageCfg.MongoDB.Database = "gomodel"
	}

	ctx := context.Background()

	// Create storage connection
	store, err := storage.New(ctx, storageCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// Create the appropriate log store based on storage type
	var logStore auditlog.LogStore
	switch store.Type() {
	case storage.TypeSQLite:
		logStore, err = auditlog.NewSQLiteStore(store.SQLiteDB(), cfg.Logging.RetentionDays)
	case storage.TypePostgreSQL:
		pool := store.PostgreSQLPool()
		if pool == nil {
			store.Close()
			return nil, nil, fmt.Errorf("PostgreSQL pool is nil")
		}
		// Use type assertion with the pgxpool.Pool type
		pgxPool, ok := pool.(*pgxpool.Pool)
		if !ok {
			store.Close()
			return nil, nil, fmt.Errorf("invalid PostgreSQL pool type: %T", pool)
		}
		logStore, err = auditlog.NewPostgreSQLStore(pgxPool, cfg.Logging.RetentionDays)
	case storage.TypeMongoDB:
		db := store.MongoDatabase()
		if db == nil {
			store.Close()
			return nil, nil, fmt.Errorf("MongoDB database is nil")
		}
		// Use type assertion with the mongo.Database type
		mongoDB, ok := db.(*mongo.Database)
		if !ok {
			store.Close()
			return nil, nil, fmt.Errorf("invalid MongoDB database type: %T", db)
		}
		logStore, err = auditlog.NewMongoDBStore(mongoDB, cfg.Logging.RetentionDays)
	default:
		store.Close()
		return nil, nil, fmt.Errorf("unknown storage type: %s", store.Type())
	}

	if err != nil {
		store.Close()
		return nil, nil, fmt.Errorf("failed to create log store: %w", err)
	}

	// Create logger configuration
	logCfg := auditlog.Config{
		Enabled:               cfg.Logging.Enabled,
		LogBodies:             cfg.Logging.LogBodies,
		LogHeaders:            cfg.Logging.LogHeaders,
		BufferSize:            cfg.Logging.BufferSize,
		FlushInterval:         time.Duration(cfg.Logging.FlushInterval) * time.Second,
		RetentionDays:         cfg.Logging.RetentionDays,
		OnlyModelInteractions: cfg.Logging.OnlyModelInteractions,
	}

	// Apply defaults
	if logCfg.BufferSize <= 0 {
		logCfg.BufferSize = 1000
	}
	if logCfg.FlushInterval <= 0 {
		logCfg.FlushInterval = 5 * time.Second
	}

	return auditlog.NewLogger(logStore, logCfg), store, nil
}
