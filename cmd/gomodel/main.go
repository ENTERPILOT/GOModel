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
	"syscall"
	"time"

	"gomodel/config"
	"gomodel/internal/auditlog"
	"gomodel/internal/observability"
	"gomodel/internal/providers"

	// Import provider packages to trigger their init() registration
	_ "gomodel/internal/providers/anthropic"
	_ "gomodel/internal/providers/gemini"
	_ "gomodel/internal/providers/groq"
	_ "gomodel/internal/providers/openai"
	_ "gomodel/internal/providers/xai"
	"gomodel/internal/server"
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

	// Setup observability hooks for metrics collection (if enabled)
	// This must be done BEFORE creating providers so they can use the hooks
	if cfg.Metrics.Enabled {
		metricsHooks := observability.NewPrometheusHooks()
		providers.SetGlobalHooks(metricsHooks)
		slog.Info("prometheus metrics enabled", "endpoint", cfg.Metrics.Endpoint)
	} else {
		slog.Info("prometheus metrics disabled")
	}

	// Initialize provider infrastructure (cache, registry, router)
	providerResult, err := providers.Init(context.Background(), cfg)
	if err != nil {
		slog.Error("failed to initialize providers", "error", err)
		os.Exit(1)
	}
	defer providerResult.Close()

	// Security check: warn if no master key is configured
	if cfg.Server.MasterKey == "" {
		slog.Warn("SECURITY WARNING: GOMODEL_MASTER_KEY not set - server running in UNSAFE MODE",
			"security_risk", "unauthenticated access allowed",
			"recommendation", "set GOMODEL_MASTER_KEY environment variable to secure this gateway")
	} else {
		slog.Info("authentication enabled", "mode", "master_key")
	}

	// Initialize audit logging
	auditResult, err := auditlog.New(context.Background(), cfg)
	if err != nil {
		slog.Error("failed to initialize audit logging", "error", err)
		os.Exit(1)
	}
	defer auditResult.Close()

	if cfg.Logging.Enabled {
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
		AuditLogger:              auditResult.Logger,
		LogOnlyModelInteractions: cfg.Logging.OnlyModelInteractions,
	}
	srv := server.New(providerResult.Router, serverCfg)

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
