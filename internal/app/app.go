// Package app provides the main application struct for centralized dependency management
// and lifecycle control of the GOModel server.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"gomodel/config"
	"gomodel/internal/auditlog"
	"gomodel/internal/core"
	"gomodel/internal/providers"
	"gomodel/internal/server"
)

// App represents the main application with all its dependencies.
// It provides centralized lifecycle management for all components.
type App struct {
	config    *config.Config
	providers *providers.InitResult
	audit     *auditlog.Result
	server    *server.Server

	shutdownMu sync.Mutex
	shutdown   bool
}

// Config holds the configuration options for creating an App.
type Config struct {
	// AppConfig is the main application configuration.
	AppConfig *config.Config

	// RefreshInterval is how often to refresh the model registry.
	// Default: 5 minutes
	RefreshInterval time.Duration

	// Factory is the provider factory with registered providers.
	// Hooks should be set on the factory before passing it here.
	Factory *providers.ProviderFactory
}

// New creates a new App with all dependencies initialized.
// The caller must call Shutdown to release resources.
func New(ctx context.Context, cfg Config) (*App, error) {
	if cfg.AppConfig == nil {
		return nil, fmt.Errorf("app config is required")
	}
	if cfg.Factory == nil {
		return nil, fmt.Errorf("factory is required")
	}

	app := &App{
		config: cfg.AppConfig,
	}

	// Initialize provider infrastructure
	initCfg := providers.InitConfig{
		RefreshInterval: cfg.RefreshInterval,
		Factory:         cfg.Factory,
	}
	if initCfg.RefreshInterval <= 0 {
		initCfg.RefreshInterval = 5 * time.Minute
	}

	providerResult, err := providers.InitWithConfig(ctx, cfg.AppConfig, initCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize providers: %w", err)
	}
	app.providers = providerResult

	// Initialize audit logging
	auditResult, err := auditlog.New(ctx, cfg.AppConfig)
	if err != nil {
		app.providers.Close()
		return nil, fmt.Errorf("failed to initialize audit logging: %w", err)
	}
	app.audit = auditResult

	// Log configuration status
	app.logStartupInfo()

	// Create server
	serverCfg := &server.Config{
		MasterKey:                cfg.AppConfig.Server.MasterKey,
		MetricsEnabled:           cfg.AppConfig.Metrics.Enabled,
		MetricsEndpoint:          cfg.AppConfig.Metrics.Endpoint,
		BodySizeLimit:            cfg.AppConfig.Server.BodySizeLimit,
		AuditLogger:              auditResult.Logger,
		LogOnlyModelInteractions: cfg.AppConfig.Logging.OnlyModelInteractions,
	}
	app.server = server.New(app.providers.Router, serverCfg)

	return app, nil
}

// Router returns the core.RoutableProvider for request routing.
func (a *App) Router() core.RoutableProvider {
	if a.providers == nil {
		return nil
	}
	return a.providers.Router
}

// AuditLogger returns the audit logger interface.
func (a *App) AuditLogger() auditlog.LoggerInterface {
	if a.audit == nil {
		return nil
	}
	return a.audit.Logger
}

// Start starts the HTTP server on the given address.
// This is a blocking call that returns when the server stops.
func (a *App) Start(addr string) error {
	slog.Info("starting server", "address", addr)
	if err := a.server.Start(addr); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			slog.Info("server stopped gracefully")
			return nil
		}
		return fmt.Errorf("server failed to start: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down all components in the correct order.
// It ensures proper cleanup of resources:
// 1. HTTP server (stop accepting new requests)
// 2. Background refresh goroutine and cache
// 3. Audit logging
//
// Safe to call multiple times; subsequent calls are no-ops.
func (a *App) Shutdown(ctx context.Context) error {
	a.shutdownMu.Lock()
	if a.shutdown {
		a.shutdownMu.Unlock()
		return nil
	}
	a.shutdown = true
	a.shutdownMu.Unlock()

	slog.Info("shutting down application...")

	var errs []error

	// 1. Shutdown HTTP server first (stop accepting new requests)
	if a.server != nil {
		if err := a.server.Shutdown(ctx); err != nil {
			slog.Error("server shutdown error", "error", err)
			errs = append(errs, fmt.Errorf("server shutdown: %w", err))
		}
	}

	// 2. Close providers (stops background refresh and cache)
	if a.providers != nil {
		if err := a.providers.Close(); err != nil {
			slog.Error("providers close error", "error", err)
			errs = append(errs, fmt.Errorf("providers close: %w", err))
		}
	}

	// 3. Close audit logging (flushes pending logs)
	if a.audit != nil {
		if err := a.audit.Close(); err != nil {
			slog.Error("audit logger close error", "error", err)
			errs = append(errs, fmt.Errorf("audit close: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %w", errors.Join(errs...))
	}

	slog.Info("application shutdown complete")
	return nil
}

// logStartupInfo logs the application configuration on startup.
func (a *App) logStartupInfo() {
	cfg := a.config

	// Security warnings
	if cfg.Server.MasterKey == "" {
		slog.Warn("SECURITY WARNING: GOMODEL_MASTER_KEY not set - server running in UNSAFE MODE",
			"security_risk", "unauthenticated access allowed",
			"recommendation", "set GOMODEL_MASTER_KEY environment variable to secure this gateway")
	} else {
		slog.Info("authentication enabled", "mode", "master_key")
	}

	// Metrics configuration
	if cfg.Metrics.Enabled {
		slog.Info("prometheus metrics enabled", "endpoint", cfg.Metrics.Endpoint)
	} else {
		slog.Info("prometheus metrics disabled")
	}

	// Audit logging configuration
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
}
