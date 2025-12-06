package main

import (
	"log/slog"
	"os"

	"heavymodel/config"
	"heavymodel/internal/providers/openai"
	"heavymodel/internal/server"
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

	// Validate API key
	if cfg.OpenAI.APIKey == "" {
		slog.Error("OPENAI_API_KEY environment variable is required")
		os.Exit(1)
	}

	// Create OpenAI provider
	provider := openai.New(cfg.OpenAI.APIKey)

	// Create and start server
	srv := server.New(provider)

	addr := ":" + cfg.Server.Port
	slog.Info("starting server", "address", addr)

	if err := srv.Start(addr); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

