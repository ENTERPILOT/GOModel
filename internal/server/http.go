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

	"gomodel/internal/core"
)

// Server wraps the Echo server
type Server struct {
	echo    *echo.Echo
	handler *Handler
}

// Config holds server configuration options
type Config struct {
	MasterKey       string // Optional: Master key for authentication
	MetricsEnabled  bool   // Whether to expose Prometheus metrics endpoint
	MetricsEndpoint string // HTTP path for metrics endpoint (default: /metrics)
}

// New creates a new HTTP server
func New(provider core.RoutableProvider, cfg *Config) *Server {
	e := echo.New()
	e.HideBanner = true

	// Global middleware (applies to all routes)
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	handler := NewHandler(provider)

	// Public routes (no authentication required)
	// These must be registered BEFORE auth middleware is applied
	e.GET("/health", handler.Health)

	// Conditionally register metrics endpoint (public, no auth)
	if cfg != nil && cfg.MetricsEnabled {
		metricsPath := cfg.MetricsEndpoint
		if metricsPath == "" {
			metricsPath = "/metrics"
		}
		// Normalize path to prevent traversal attacks (e.g., /v1/../admin -> /admin)
		// and then validate it doesn't shadow protected API routes
		metricsPath = path.Clean(metricsPath)
		if metricsPath == "/v1" || strings.HasPrefix(metricsPath, "/v1/") {
			slog.Warn("metrics endpoint path conflicts with API routes, using /metrics instead",
				"configured_path", cfg.MetricsEndpoint,
				"normalized_path", metricsPath)
			metricsPath = "/metrics"
		}
		e.GET(metricsPath, echo.WrapHandler(promhttp.Handler()))
	}

	// API routes group with authentication and body size limit
	api := e.Group("/v1")

	// Add body size limit to prevent DoS (10MB max)
	api.Use(middleware.BodyLimit("10M"))

	// Add authentication middleware if master key is configured
	if cfg != nil && cfg.MasterKey != "" {
		api.Use(AuthMiddleware(cfg.MasterKey))
	}

	api.GET("/models", handler.ListModels)
	api.POST("/chat/completions", handler.ChatCompletion)
	api.POST("/responses", handler.Responses)

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
