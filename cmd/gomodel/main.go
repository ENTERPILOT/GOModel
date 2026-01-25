// Package main is the entry point for the LLM gateway server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gomodel/config"
	"gomodel/internal/app"
	"gomodel/internal/observability"
	"gomodel/internal/providers"
	"gomodel/internal/providers/anthropic"
	"gomodel/internal/providers/gemini"
	"gomodel/internal/providers/groq"
	"gomodel/internal/providers/ollama"
	"gomodel/internal/providers/openai"
	"gomodel/internal/providers/xai"
	"gomodel/internal/version"
)

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

	// Create provider factory and register all providers explicitly
	factory := providers.NewProviderFactory()

	// Set observability hooks before registering providers
	if cfg.Metrics.Enabled {
		factory.SetHooks(observability.NewPrometheusHooks())
	}

	// Register all providers with the factory
	factory.Register(openai.Registration)
	factory.Register(anthropic.Registration)
	factory.Register(gemini.Registration)
	factory.Register(groq.Registration)
	factory.Register(ollama.Registration)
	factory.Register(xai.Registration)

	// Create the application
	application, err := app.New(context.Background(), app.Config{
		AppConfig: cfg,
		Factory:   factory,
	})
	if err != nil {
		slog.Error("failed to initialize application", "error", err)
		os.Exit(1)
	}

	// Handle graceful shutdown
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := application.Shutdown(ctx); err != nil {
			slog.Error("application shutdown error", "error", err)
		}
	}()

	// Start the server (blocking)
	addr := ":" + cfg.Server.Port
	if err := application.Start(addr); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}
