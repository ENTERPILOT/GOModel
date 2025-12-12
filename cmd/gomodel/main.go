// Package main is the entry point for the LLM gateway server.
package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gomodel/config"
	"gomodel/internal/providers"

	// Import provider packages to trigger their init() registration
	_ "gomodel/internal/providers/anthropic"
	_ "gomodel/internal/providers/gemini"
	_ "gomodel/internal/providers/openai"
	"gomodel/internal/server"
)

// getCacheDir returns the directory for cache files.
// Uses $GOMODEL_CACHE_DIR if set, otherwise ./.cache (working directory)
func getCacheDir() string {
	if cacheDir := os.Getenv("GOMODEL_CACHE_DIR"); cacheDir != "" {
		return cacheDir
	}
	return ".cache"
}

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

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

	// Create model registry with cache file for instant startup
	registry := providers.NewModelRegistry()
	cacheFile := filepath.Join(getCacheDir(), "models.json")
	registry.SetCacheFile(cacheFile)
	slog.Debug("cache file configured", "path", cacheFile)

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

	slog.Info("model registry starting",
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

	// Create and start server
	srv := server.New(router)

	addr := ":" + cfg.Server.Port
	slog.Info("starting server", "address", addr)

	if err := srv.Start(addr); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
