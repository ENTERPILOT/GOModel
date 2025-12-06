package server

import (
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
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body: " + err.Error(),
		})
	}

	if !h.provider.Supports(req.Model) {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "unsupported model: " + req.Model,
		})
	}

	// Handle streaming: proxy the raw SSE stream
	if req.Stream {
		stream, err := h.provider.StreamChatCompletion(c.Request().Context(), &req)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		defer stream.Close()

		c.Response().Header().Set("Content-Type", "text/event-stream")
		c.Response().Header().Set("Cache-Control", "no-cache")
		c.Response().Header().Set("Connection", "keep-alive")
		c.Response().WriteHeader(http.StatusOK)

		io.Copy(c.Response().Writer, stream)
		return nil
	}

	// Non-streaming
	resp, err := h.provider.ChatCompletion(c.Request().Context(), &req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, resp)
}
