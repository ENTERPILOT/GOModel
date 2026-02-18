package providers

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"time"

	"gomodel/config"
	"gomodel/internal/cache"
	"gomodel/internal/core"
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

// Init initializes the provider registry, cache, and router.
//
// It performs:
// 1. Cache initialization (local or Redis based on config)
// 2. Provider creation and registration
// 3. Async model loading (from cache first, then network refresh)
// 4. Background refresh scheduling (hourly)
// 5. Router creation
//
// The caller must call InitResult.Close() during shutdown.
func Init(ctx context.Context, cfg *config.Config, factory *ProviderFactory) (*InitResult, error) {
	if factory == nil {
		return nil, fmt.Errorf("factory is required")
	}

	modelCache, err := initCache(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache: %w", err)
	}

	registry := NewModelRegistry()
	registry.SetCache(modelCache)

	count, err := registerProviders(cfg, factory, registry)
	if err != nil {
		modelCache.Close()
		return nil, err
	}
	if count == 0 {
		modelCache.Close()
		return nil, fmt.Errorf("no providers were successfully initialized")
	}

	slog.Info("starting non-blocking model registry initialization...")
	registry.InitializeAsync(ctx)

	slog.Info("model registry configured",
		"cached_models", registry.ModelCount(),
		"providers", registry.ProviderCount(),
	)

	refreshInterval := time.Duration(cfg.Cache.RefreshInterval) * time.Second
	if refreshInterval <= 0 {
		refreshInterval = time.Hour
	}
	stopRefresh := registry.StartBackgroundRefresh(refreshInterval)

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
		cacheDir := cfg.Cache.CacheDir
		if cacheDir == "" {
			cacheDir = ".cache"
		}
		cacheFile := filepath.Join(cacheDir, "models.json")
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

		// Check availability for providers that support it
		if checker, ok := p.(core.AvailabilityChecker); ok {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := checker.CheckAvailability(ctx); err != nil {
				slog.Info("provider not available, skipping",
					"name", name,
					"type", pCfg.Type,
					"reason", err.Error())
				cancel()
				continue
			}
			cancel()
		}

		registry.RegisterProviderWithType(p, pCfg.Type)
		initializedCount++
		slog.Info("provider initialized", "name", name, "type", pCfg.Type)
	}

	return initializedCount, nil
}
