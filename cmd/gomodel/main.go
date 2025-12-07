// Package main is the entry point for the LLM gateway server.
package main

import (
	"log/slog"
	"os"

	"gomodel/config"
	"gomodel/internal/core"
	"gomodel/internal/providers"
	"gomodel/internal/providers/anthropic"
	"gomodel/internal/providers/gemini"
	"gomodel/internal/providers/openai"
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

	// Validate that at least one API key is provided
	if cfg.OpenAI.APIKey == "" && cfg.Anthropic.APIKey == "" && cfg.Gemini.APIKey == "" {
		slog.Error("at least one API key is required (OPENAI_API_KEY, ANTHROPIC_API_KEY, or GEMINI_API_KEY)")
		os.Exit(1)
	}

	// Create providers
	providerList := make([]core.Provider, 0, 3)

	if cfg.OpenAI.APIKey != "" {
		openaiProvider := openai.New(cfg.OpenAI.APIKey)
		providerList = append(providerList, openaiProvider)
		slog.Info("OpenAI provider initialized")
	}

	if cfg.Anthropic.APIKey != "" {
		anthropicProvider := anthropic.New(cfg.Anthropic.APIKey)
		providerList = append(providerList, anthropicProvider)
		slog.Info("Anthropic provider initialized")
	}

	if cfg.Gemini.APIKey != "" {
		geminiProvider := gemini.New(cfg.Gemini.APIKey)
		providerList = append(providerList, geminiProvider)
		slog.Info("Gemini provider initialized")
	}

	// Create provider router
	provider := providers.NewRouter(providerList...)

	// Create and start server
	srv := server.New(provider)

	addr := ":" + cfg.Server.Port
	slog.Info("starting server", "address", addr)

	if err := srv.Start(addr); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
