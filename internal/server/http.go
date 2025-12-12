package server

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"gomodel/internal/core"
)

// Server wraps the Echo server
type Server struct {
	echo    *echo.Echo
	handler *Handler
}

// New creates a new HTTP server
func New(provider core.Provider) *Server {
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	handler := NewHandler(provider)

	// Routes
	e.GET("/health", handler.Health)
	e.GET("/v1/models", handler.ListModels)
	e.POST("/v1/chat/completions", handler.ChatCompletion)

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
