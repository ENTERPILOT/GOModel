package server

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"gomodel/internal/core"
)

// Server wraps the Echo server
type Server struct {
	echo    *echo.Echo
	handler *Handler
}

// Config holds server configuration options
type Config struct {
	MasterKey string // Optional: Master key for authentication
}

// New creates a new HTTP server
func New(provider core.RoutableProvider, cfg *Config) *Server {
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Add authentication middleware if master key is configured
	if cfg != nil && cfg.MasterKey != "" {
		e.Use(AuthMiddleware(cfg.MasterKey))
	}

	handler := NewHandler(provider)

	// Routes
	e.GET("/health", handler.Health)
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
