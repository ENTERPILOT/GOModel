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
	"gomodel/internal/admin"
	"gomodel/internal/admin/dashboard"
	"gomodel/internal/auditlog"
	"gomodel/internal/core"
	"gomodel/internal/guardrails"
	"gomodel/internal/providers"
	"gomodel/internal/server"
	"gomodel/internal/storage"
	"gomodel/internal/usage"
)

// App represents the main application with all its dependencies.
// It provides centralized lifecycle management for all components.
type App struct {
	config    *config.Config
	providers *providers.InitResult
	audit     *auditlog.Result
	usage     *usage.Result
	server    *server.Server

	shutdownMu sync.Mutex
	shutdown   bool
}

// Config holds the configuration options for creating an App.
type Config struct {
	// AppConfig is the main application configuration.
	AppConfig *config.Config

	// RefreshInterval is how often to refresh the model registry.
	// Default: 1 hour
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
	// RefreshInterval default (1 hour) is applied in providers.InitWithConfig if zero
	initCfg := providers.InitConfig{
		RefreshInterval: cfg.RefreshInterval,
		Factory:         cfg.Factory,
	}

	providerResult, err := providers.InitWithConfig(ctx, cfg.AppConfig, initCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize providers: %w", err)
	}
	app.providers = providerResult

	// Initialize audit logging
	auditResult, err := auditlog.New(ctx, cfg.AppConfig)
	if err != nil {
		closeErr := app.providers.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("failed to initialize audit logging: %w (also: providers close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to initialize audit logging: %w", err)
	}
	app.audit = auditResult

	// Initialize usage tracking
	// Use shared storage if both audit logging and usage tracking use the same backend
	var usageResult *usage.Result
	if auditResult.Storage != nil && cfg.AppConfig.Usage.Enabled {
		// Share storage connection with audit logging
		usageResult, err = usage.NewWithSharedStorage(ctx, cfg.AppConfig, auditResult.Storage)
	} else {
		// Create separate storage or return noop logger
		usageResult, err = usage.New(ctx, cfg.AppConfig)
	}
	if err != nil {
		closeErr := errors.Join(app.audit.Close(), app.providers.Close())
		if closeErr != nil {
			return nil, fmt.Errorf("failed to initialize usage tracking: %w (also: close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to initialize usage tracking: %w", err)
	}
	app.usage = usageResult

	// Log configuration status
	app.logStartupInfo()

	// Build the provider chain: router optionally wrapped with guardrails
	var provider core.RoutableProvider = app.providers.Router
	if cfg.AppConfig.Guardrails.Enabled {
		pipeline, err := buildGuardrailsPipeline(cfg.AppConfig.Guardrails)
		if err != nil {
			closeErr := errors.Join(app.usage.Close(), app.audit.Close(), app.providers.Close())
			if closeErr != nil {
				return nil, fmt.Errorf("failed to build guardrails: %w (also: close error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to build guardrails: %w", err)
		}
		if pipeline.Len() > 0 {
			provider = guardrails.NewGuardedProvider(provider, pipeline)
			slog.Info("guardrails enabled", "count", pipeline.Len())
		}
	}

	// Create server
	serverCfg := &server.Config{
		MasterKey:                cfg.AppConfig.Server.MasterKey,
		MetricsEnabled:           cfg.AppConfig.Metrics.Enabled,
		MetricsEndpoint:          cfg.AppConfig.Metrics.Endpoint,
		BodySizeLimit:            cfg.AppConfig.Server.BodySizeLimit,
		AuditLogger:              auditResult.Logger,
		UsageLogger:              usageResult.Logger,
		LogOnlyModelInteractions: cfg.AppConfig.Logging.OnlyModelInteractions,
	}

	// Initialize admin API and dashboard (behind separate feature flags)
	adminCfg := cfg.AppConfig.Admin
	if !adminCfg.EndpointsEnabled && adminCfg.UIEnabled {
		slog.Warn("ADMIN_UI_ENABLED=true requires ADMIN_ENDPOINTS_ENABLED=true — forcing UI to disabled")
		adminCfg.UIEnabled = false
	}
	if adminCfg.EndpointsEnabled {
		adminHandler, dashHandler, adminErr := initAdmin(auditResult.Storage, usageResult.Storage, providerResult.Registry, adminCfg.UIEnabled)
		if adminErr != nil {
			slog.Warn("failed to initialize admin", "error", adminErr)
		} else {
			serverCfg.AdminEndpointsEnabled = true
			serverCfg.AdminHandler = adminHandler
			if adminCfg.UIEnabled {
				serverCfg.AdminUIEnabled = true
				serverCfg.DashboardHandler = dashHandler
			}
		}
	}

	app.server = server.New(provider, serverCfg)

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

// UsageLogger returns the usage logger interface.
func (a *App) UsageLogger() usage.LoggerInterface {
	if a.usage == nil {
		return nil
	}
	return a.usage.Logger
}

// Start starts the HTTP server on the given address.
// This is a blocking call that returns when the server stops.
func (a *App) Start(addr string) error {
	if a.server == nil {
		return fmt.Errorf("server is not initialized")
	}
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

	// 3. Close usage tracking (flushes pending entries)
	if a.usage != nil {
		if err := a.usage.Close(); err != nil {
			slog.Error("usage logger close error", "error", err)
			errs = append(errs, fmt.Errorf("usage close: %w", err))
		}
	}

	// 4. Close audit logging (flushes pending logs)
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

	// Storage configuration (shared by audit logging and usage tracking)
	slog.Info("storage configured", "type", cfg.Storage.Type)

	// Audit logging configuration
	if cfg.Logging.Enabled {
		slog.Info("audit logging enabled",
			"log_bodies", cfg.Logging.LogBodies,
			"log_headers", cfg.Logging.LogHeaders,
			"retention_days", cfg.Logging.RetentionDays,
		)
	} else {
		slog.Info("audit logging disabled")
	}

	// Usage tracking configuration
	if cfg.Usage.Enabled {
		slog.Info("usage tracking enabled",
			"buffer_size", cfg.Usage.BufferSize,
			"flush_interval", cfg.Usage.FlushInterval,
			"retention_days", cfg.Usage.RetentionDays,
		)
	} else {
		slog.Info("usage tracking disabled")
	}

	// Admin configuration
	if cfg.Admin.EndpointsEnabled {
		slog.Info("admin API enabled", "api", "/admin/api/v1")
	} else {
		slog.Info("admin API disabled")
	}
	if cfg.Admin.UIEnabled && cfg.Admin.EndpointsEnabled {
		slog.Info("admin UI enabled", "url", "/admin/dashboard")
	} else {
		slog.Info("admin UI disabled")
	}
}

// initAdmin creates the admin API handler and optionally the dashboard handler.
// Returns nil dashboard handler if uiEnabled is false.
func initAdmin(auditStorage, usageStorage storage.Storage, registry *providers.ModelRegistry, uiEnabled bool) (*admin.Handler, *dashboard.Handler, error) {
	// Find a storage connection for reading usage data
	var store storage.Storage
	if auditStorage != nil {
		store = auditStorage
	} else if usageStorage != nil {
		store = usageStorage
	}

	// Create usage reader (may be nil if no storage)
	var reader usage.UsageReader
	if store != nil {
		var err error
		reader, err = usage.NewReader(store)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create usage reader: %w", err)
		}
	}

	adminHandler := admin.NewHandler(reader, registry)

	var dashHandler *dashboard.Handler
	if uiEnabled {
		var err error
		dashHandler, err = dashboard.New()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to initialize dashboard: %w", err)
		}
	}

	return adminHandler, dashHandler, nil
}

// buildGuardrailsPipeline creates a guardrails pipeline from configuration.
func buildGuardrailsPipeline(cfg config.GuardrailsConfig) (*guardrails.Pipeline, error) {
	pipeline := guardrails.NewPipeline()

	for i, rule := range cfg.Rules {
		g, err := buildGuardrail(rule)
		if err != nil {
			return nil, fmt.Errorf("guardrail rule #%d (%q): %w", i, rule.Name, err)
		}
		pipeline.Add(g, rule.Order)
		slog.Info("guardrail registered", "name", rule.Name, "type", rule.Type, "order", rule.Order)
	}

	return pipeline, nil
}

// buildGuardrail creates a single Guardrail instance from a rule config.
func buildGuardrail(rule config.GuardrailRuleConfig) (guardrails.Guardrail, error) {
	if rule.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	switch rule.Type {
	case "system_prompt":
		mode := guardrails.SystemPromptMode(rule.SystemPrompt.Mode)
		if mode == "" {
			mode = guardrails.SystemPromptInject
		}
		return guardrails.NewSystemPromptGuardrail(rule.Name, mode, rule.SystemPrompt.Content)

	default:
		return nil, fmt.Errorf("unknown guardrail type: %q", rule.Type)
	}
}
