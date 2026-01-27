package server

import (
	"context"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"gomodel/internal/auditlog"
	"gomodel/internal/core"
	"gomodel/internal/guardrails"
	"gomodel/internal/usage"
)

// Server wraps the Echo server
type Server struct {
	echo    *echo.Echo
	handler *Handler
}

// Config holds server configuration options
type Config struct {
	MasterKey                string                   // Optional: Master key for authentication
	MetricsEnabled           bool                     // Whether to expose Prometheus metrics endpoint
	MetricsEndpoint          string                   // HTTP path for metrics endpoint (default: /metrics)
	BodySizeLimit            string                   // Max request body size (e.g., "10M", "1024K")
	AuditLogger              auditlog.LoggerInterface // Optional: Audit logger for request/response logging
	UsageLogger              usage.LoggerInterface    // Optional: Usage logger for token tracking
	LogOnlyModelInteractions bool                     // Only log AI model endpoints (default: true)
	Guardrails               *guardrails.Processor    // Optional: Guardrails processor for request preprocessing
}

// New creates a new HTTP server
func New(provider core.RoutableProvider, cfg *Config) *Server {
	e := echo.New()
	e.HideBanner = true

	// Get loggers and guardrails from config (may be nil)
	var auditLogger auditlog.LoggerInterface
	var usageLogger usage.LoggerInterface
	var guardrailsProcessor *guardrails.Processor
	if cfg != nil {
		auditLogger = cfg.AuditLogger
		usageLogger = cfg.UsageLogger
		guardrailsProcessor = cfg.Guardrails
	}

	handler := NewHandler(provider, auditLogger, usageLogger, guardrailsProcessor)

	// Build list of paths that skip authentication
	authSkipPaths := []string{"/health"}

	// Determine metrics path
	metricsPath := "/metrics"
	if cfg != nil && cfg.MetricsEnabled {
		if cfg.MetricsEndpoint != "" {
			// Normalize path to prevent traversal attacks
			metricsPath = path.Clean(cfg.MetricsEndpoint)
		}
		// Prevent metrics endpoint from shadowing API routes (security: auth bypass)
		if metricsPath == "/v1" || strings.HasPrefix(metricsPath, "/v1/") {
			slog.Warn("metrics endpoint conflicts with API routes, using /metrics instead",
				"configured", cfg.MetricsEndpoint,
				"normalized", metricsPath)
			metricsPath = "/metrics"
		}
		authSkipPaths = append(authSkipPaths, metricsPath)
	}

	// Global middleware stack (order matters)
	// Request logger with optional filtering for model-only interactions
	if cfg != nil && cfg.LogOnlyModelInteractions {
		e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
			Skipper: func(c echo.Context) bool {
				return !auditlog.IsModelInteractionPath(c.Request().URL.Path)
			},
			LogStatus:   true,
			LogURI:      true,
			LogError:    true,
			LogMethod:   true,
			LogLatency:  true,
			LogProtocol: true,
			LogRemoteIP: true,
			LogHost:     true,
			LogURIPath:  true,
			LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
				slog.Info("REQUEST",
					"method", v.Method,
					"uri", v.URI,
					"status", v.Status,
					"latency", v.Latency.String(),
					"host", v.Host,
					"bytes_in", c.Request().ContentLength,
					"bytes_out", c.Response().Size,
					"user_agent", c.Request().UserAgent(),
					"remote_ip", v.RemoteIP,
					"request_id", c.Request().Header.Get("X-Request-ID"),
				)
				return nil
			},
		}))
	} else {
		e.Use(middleware.RequestLogger())
	}
	e.Use(middleware.Recover())

	// Body size limit (default: 10MB)
	bodySizeLimit := "10M"
	if cfg != nil && cfg.BodySizeLimit != "" {
		bodySizeLimit = cfg.BodySizeLimit
	}
	e.Use(middleware.BodyLimit(bodySizeLimit))

	// Audit logging middleware (before authentication to capture all requests)
	if cfg != nil && cfg.AuditLogger != nil && cfg.AuditLogger.Config().Enabled {
		e.Use(auditlog.Middleware(cfg.AuditLogger))
	}

	// Authentication (skips public paths)
	if cfg != nil && cfg.MasterKey != "" {
		e.Use(AuthMiddleware(cfg.MasterKey, authSkipPaths))
	}

	// Public routes
	e.GET("/health", handler.Health)
	if cfg != nil && cfg.MetricsEnabled {
		e.GET(metricsPath, echo.WrapHandler(promhttp.Handler()))
	}

	// API routes
	e.GET("/v1/models", handler.ListModels)
	e.POST("/v1/chat/completions", handler.ChatCompletion)
	e.POST("/v1/responses", handler.Responses)

	return &Server{
		echo:    e,
		handler: handler,
	}
}

// Start starts the HTTP server on the given address
func (s *Server) Start(addr string) error {
	return s.echo.Start(addr)
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.echo.Shutdown(ctx)
}

// ServeHTTP implements the http.Handler interface, allowing Server to be used with httptest
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.echo.ServeHTTP(w, r)
}
