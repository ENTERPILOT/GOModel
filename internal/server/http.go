package server

import (
	"context"
	"net/http"
	"path"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"gomodel/config"
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
	BodySizeLimit   int64  // Max request body size in bytes (default: 10MB)
}

// New creates a new HTTP server
func New(provider core.RoutableProvider, cfg *Config) *Server {
	e := echo.New()
	e.HideBanner = true

	handler := NewHandler(provider)

	// Build list of paths that skip authentication
	authSkipPaths := []string{"/health"}

	// Determine metrics path
	metricsPath := "/metrics"
	if cfg != nil && cfg.MetricsEnabled {
		if cfg.MetricsEndpoint != "" {
			// Normalize path to prevent traversal attacks
			metricsPath = path.Clean(cfg.MetricsEndpoint)
		}
		authSkipPaths = append(authSkipPaths, metricsPath)
	}

	// Global middleware stack (order matters)
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	// Body size limit (default: 10MB)
	bodySizeLimit := config.DefaultBodySizeLimit
	if cfg != nil && cfg.BodySizeLimit > 0 {
		bodySizeLimit = cfg.BodySizeLimit
	}
	e.Use(middleware.BodyLimit(strconv.FormatInt(bodySizeLimit, 10)))

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
