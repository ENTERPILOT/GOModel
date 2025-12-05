package server

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"heavymodel/internal/core"
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

	// Check if provider supports the model
	if !h.provider.Supports(req.Model) {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "unsupported model: " + req.Model,
		})
	}

	// Execute the chat completion
	resp, err := h.provider.ChatCompletion(c.Request().Context(), &req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, resp)
}

// Health handles GET /health
func (h *Handler) Health(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status": "ok",
	})
}

