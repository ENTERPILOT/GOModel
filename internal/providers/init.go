package providers

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gomodel/config"
	"gomodel/internal/cache"
)

// InitResult holds the initialized provider infrastructure and cleanup functions.
type InitResult struct {
	Registry *ModelRegistry
	Router   *Router
	Cache    cache.Cache
	Factory  *ProviderFactory

	// stopRefresh is called to stop the background refresh goroutine
	stopRefresh func()
}

// Close releases all resources and stops background goroutines.
// Safe to call multiple times (but stopRefresh is only called once).
func (r *InitResult) Close() error {
	if r.stopRefresh != nil {
		r.stopRefresh()
		r.stopRefresh = nil // Prevent double-call
	}
	if r.Cache != nil {
		return r.Cache.Close()
	}
	return nil
}

// InitConfig holds options for provider initialization.
type InitConfig struct {
	// RefreshInterval is how often to refresh the model registry.
	// Default: 5 minutes
	RefreshInterval time.Duration

	// Factory is the provider factory with registered providers.
	// Hooks should be set on the factory before passing it here.
	Factory *ProviderFactory
}

// DefaultInitConfig returns sensible defaults for initialization.
func DefaultInitConfig() InitConfig {
	return InitConfig{
		RefreshInterval: 5 * time.Minute,
	}
}

// Init initializes the provider registry, cache, and router.
// This is the main entry point for provider infrastructure setup.
//
// It performs:
// 1. Cache initialization (local or Redis based on config)
// 2. Provider creation and registration
// 3. Async model loading (from cache first, then network refresh)
// 4. Background refresh scheduling
// 5. Router creation
//
// The caller must call InitResult.Close() during shutdown.
func Init(ctx context.Context, cfg *config.Config) (*InitResult, error) {
	return InitWithConfig(ctx, cfg, DefaultInitConfig())
}

// InitWithConfig initializes the provider infrastructure with custom options.
func InitWithConfig(ctx context.Context, cfg *config.Config, initCfg InitConfig) (*InitResult, error) {
	// Validate that factory is provided
	if initCfg.Factory == nil {
		return nil, fmt.Errorf("InitConfig.Factory is required")
	}

	// Initialize cache backend based on configuration
	modelCache, err := initCache(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache: %w", err)
	}

	factory := initCfg.Factory

	// Create model registry with cache
	registry := NewModelRegistry()
	registry.SetCache(modelCache)

	// Register providers using the factory
	count, err := registerProviders(cfg, factory, registry)
	if err != nil {
		modelCache.Close()
		return nil, err
	}
	if count == 0 {
		modelCache.Close()
		return nil, fmt.Errorf("no providers were successfully initialized")
	}

	// Non-blocking initialization: load from cache, then refresh in background
	slog.Info("starting non-blocking model registry initialization...")
	registry.InitializeAsync(ctx)

	slog.Info("model registry configured",
		"cached_models", registry.ModelCount(),
		"providers", registry.ProviderCount(),
	)

	// Start background refresh
	interval := initCfg.RefreshInterval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	stopRefresh := registry.StartBackgroundRefresh(interval)

	// Create router
	router, err := NewRouter(registry)
	if err != nil {
		stopRefresh()
		modelCache.Close()
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	return &InitResult{
		Registry:    registry,
		Router:      router,
		Cache:       modelCache,
		Factory:     factory,
		stopRefresh: stopRefresh,
	}, nil
}

// getCacheDir returns the directory for cache files.
// Uses $GOMODEL_CACHE_DIR if set, otherwise ./.cache (working directory)
func getCacheDir() string {
	if cacheDir := os.Getenv("GOMODEL_CACHE_DIR"); cacheDir != "" {
		return cacheDir
	}
	return ".cache"
}

// initCache initializes the appropriate cache backend based on configuration.
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

// registerProviders creates and registers all configured providers.
// Returns the count of successfully initialized providers.
func registerProviders(cfg *config.Config, factory *ProviderFactory, registry *ModelRegistry) (int, error) {
	// Sort provider names for deterministic initialization order
	providerNames := make([]string, 0, len(cfg.Providers))
	for name := range cfg.Providers {
		providerNames = append(providerNames, name)
	}
	sort.Strings(providerNames)

	var initializedCount int
	for _, name := range providerNames {
		pCfg := cfg.Providers[name]
		p, err := factory.Create(pCfg)
		if err != nil {
			slog.Error("failed to initialize provider",
				"name", name,
				"type", pCfg.Type,
				"error", err)
			continue
		}
		registry.RegisterProviderWithType(p, pCfg.Type)
		initializedCount++
		slog.Info("provider initialized", "name", name, "type", pCfg.Type)
	}

	return initializedCount, nil
}
