// Package main is the entry point for the LLM gateway server.
package main

import (
	"context"
	"log/slog"
	"os"
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

	// Create model registry
	registry := providers.NewModelRegistry()

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
		registry.RegisterProvider(p)
		initializedCount++
		slog.Info("provider initialized", "name", name, "type", pCfg.Type)
	}

	// Validate that at least one provider was successfully initialized
	if initializedCount == 0 {
		slog.Error("no providers were successfully initialized")
		os.Exit(1)
	}

	// Initialize model registry by fetching models from all providers
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	slog.Info("initializing model registry...")
	if err := registry.Initialize(ctx); err != nil {
		slog.Error("failed to initialize model registry", "error", err)
		os.Exit(1)
	}

	slog.Info("model registry ready",
		"models", registry.ModelCount(),
		"providers", registry.ProviderCount(),
	)

	// Optional: Start background refresh of model registry (every 5 minutes)
	// This keeps the model list up-to-date as providers add/remove models
	stopRefresh := registry.StartBackgroundRefresh(5 * time.Minute)
	defer stopRefresh()

	// Create provider router
	router := providers.NewRouter(registry)

	// Create and start server
	srv := server.New(router)

	addr := ":" + cfg.Server.Port
	slog.Info("starting server", "address", addr)

	if err := srv.Start(addr); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
