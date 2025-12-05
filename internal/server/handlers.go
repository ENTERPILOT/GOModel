// Package server provides HTTP handlers and server setup for the LLM gateway.
package server

import (
	"errors"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"gomodel/internal/core"
)

// Handler holds the HTTP handlers
type Handler struct {
	provider core.Provider
}

// NewHandler creates a new handler with the given provider
func NewHandler(provider core.Provider) *Handler {
	return &Handler{
		provider: provider,
	}
}

// ChatCompletion handles POST /v1/chat/completions
func (h *Handler) ChatCompletion(c echo.Context) error {
	var req core.ChatRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}

	if !h.provider.Supports(req.Model) {
		return handleError(c, core.NewInvalidRequestError("unsupported model: "+req.Model, nil))
	}

	// Handle streaming: proxy the raw SSE stream
	if req.Stream {
		stream, err := h.provider.StreamChatCompletion(c.Request().Context(), &req)
		if err != nil {
			return handleError(c, err)
		}
		defer func() {
			_ = stream.Close() //nolint:errcheck
		}()

		c.Response().Header().Set("Content-Type", "text/event-stream")
		c.Response().Header().Set("Cache-Control", "no-cache")
		c.Response().Header().Set("Connection", "keep-alive")
		c.Response().WriteHeader(http.StatusOK)

		if _, err := io.Copy(c.Response().Writer, stream); err != nil {
			// Can't return error after headers are sent, log it
			return nil
		}
		return nil
	}

	// Non-streaming
	resp, err := h.provider.ChatCompletion(c.Request().Context(), &req)
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(http.StatusOK, resp)
}

// Health handles GET /health
func (h *Handler) Health(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ListModels handles GET /v1/models
func (h *Handler) ListModels(c echo.Context) error {
	resp, err := h.provider.ListModels(c.Request().Context())
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(http.StatusOK, resp)
}

// handleError converts gateway errors to appropriate HTTP responses
func handleError(c echo.Context, err error) error {
	var gatewayErr *core.GatewayError
	if errors.As(err, &gatewayErr) {
		return c.JSON(gatewayErr.HTTPStatusCode(), gatewayErr.ToJSON())
	}

	// Fallback for unexpected errors
	return c.JSON(http.StatusInternalServerError, map[string]interface{}{
		"error": map[string]interface{}{
			"type":    "internal_error",
			"message": "an unexpected error occurred",
		},
	})
}
