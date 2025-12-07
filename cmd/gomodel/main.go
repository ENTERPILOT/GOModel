// Package main is the entry point for the LLM gateway server.
package main

import (
	"log/slog"
	"os"

	"gomodel/config"
	"gomodel/internal/core"
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

	// Create providers dynamically using the factory
	activeProviders := make([]core.Provider, 0, len(cfg.Providers))

	for name, pCfg := range cfg.Providers {
		p, err := providers.Create(pCfg)
		if err != nil {
			slog.Error("failed to initialize provider", "name", name, "type", pCfg.Type, "error", err)
			continue
		}
		activeProviders = append(activeProviders, p)
		slog.Info("provider initialized", "name", name, "type", pCfg.Type)
	}

	// Validate that at least one provider was successfully initialized
	if len(activeProviders) == 0 {
		slog.Error("no providers were successfully initialized")
		os.Exit(1)
	}

	// Create provider router
	provider := providers.NewRouter(activeProviders...)

	// Create and start server
	srv := server.New(provider)

	addr := ":" + cfg.Server.Port
	slog.Info("starting server", "address", addr)

	if err := srv.Start(addr); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
