// Package server provides HTTP handlers and server setup for the LLM gateway.
package server

import (
	"errors"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"gomodel/internal/auditlog"
	"gomodel/internal/core"
	"gomodel/internal/usage"
)

// Handler holds the HTTP handlers
type Handler struct {
	provider    core.RoutableProvider
	logger      auditlog.LoggerInterface
	usageLogger usage.LoggerInterface
}

// NewHandler creates a new handler with the given routable provider (typically the Router)
func NewHandler(provider core.RoutableProvider, logger auditlog.LoggerInterface, usageLogger usage.LoggerInterface) *Handler {
	return &Handler{
		provider:    provider,
		logger:      logger,
		usageLogger: usageLogger,
	}
}

// handleStreamingResponse handles SSE streaming responses for both ChatCompletion and Responses endpoints.
// It wraps the stream with audit logging and usage tracking, and sets appropriate SSE headers.
func (h *Handler) handleStreamingResponse(c echo.Context, model, provider string, streamFn func() (io.ReadCloser, error)) error {
	// Call streamFn first - only mark as streaming after success
	// This ensures failed streams are logged normally by handleError/middleware
	stream, err := streamFn()
	if err != nil {
		return handleError(c, err)
	}

	// Mark as streaming so middleware doesn't log (StreamLogWrapper handles it)
	auditlog.MarkEntryAsStreaming(c, true)
	auditlog.EnrichEntryWithStream(c, true)

	// Get entry from context and wrap stream for logging
	entry := auditlog.GetStreamEntryFromContext(c)
	streamEntry := auditlog.CreateStreamEntry(entry)
	if streamEntry != nil {
		streamEntry.StatusCode = http.StatusOK // Streaming always starts with 200 OK
	}
	wrappedStream := auditlog.WrapStreamForLogging(stream, h.logger, streamEntry, c.Request().URL.Path)

	// Wrap with usage tracking if enabled
	requestID := c.Response().Header().Get("X-Request-ID")
	endpoint := c.Request().URL.Path
	wrappedStream = usage.WrapStreamForUsage(wrappedStream, h.usageLogger, model, provider, requestID, endpoint)

	defer func() {
		_ = wrappedStream.Close() //nolint:errcheck
	}()

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")

	// Capture response headers on stream entry AFTER setting them
	if streamEntry != nil && streamEntry.Data != nil {
		streamEntry.Data.ResponseHeaders = map[string]string{
			"Content-Type":  "text/event-stream",
			"Cache-Control": "no-cache",
			"Connection":    "keep-alive",
		}
	}

	c.Response().WriteHeader(http.StatusOK)
	_, _ = io.Copy(c.Response().Writer, wrappedStream)
	return nil
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

	// Enrich audit log entry with model and provider
	providerType := h.provider.GetProviderType(req.Model)
	auditlog.EnrichEntry(c, req.Model, providerType)

	// Handle streaming: proxy the raw SSE stream
	if req.Stream {
		return h.handleStreamingResponse(c, req.Model, providerType, func() (io.ReadCloser, error) {
			return h.provider.StreamChatCompletion(c.Request().Context(), &req)
		})
	}

	// Non-streaming
	resp, err := h.provider.ChatCompletion(c.Request().Context(), &req)
	if err != nil {
		return handleError(c, err)
	}

	// Track usage if enabled
	if h.usageLogger != nil && h.usageLogger.Config().Enabled {
		requestID := c.Response().Header().Get("X-Request-ID")
		usageEntry := usage.ExtractFromChatResponse(resp, requestID, "/v1/chat/completions")
		if usageEntry != nil {
			h.usageLogger.Write(usageEntry)
		}
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

// Responses handles POST /v1/responses
func (h *Handler) Responses(c echo.Context) error {
	var req core.ResponsesRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}

	if req.Model == "" {
		return handleError(c, core.NewInvalidRequestError("model is required", nil))
	}

	if !h.provider.Supports(req.Model) {
		return handleError(c, core.NewInvalidRequestError("unsupported model: "+req.Model, nil))
	}

	// Enrich audit log entry with model and provider
	providerType := h.provider.GetProviderType(req.Model)
	auditlog.EnrichEntry(c, req.Model, providerType)

	// Handle streaming: proxy the raw SSE stream
	if req.Stream {
		return h.handleStreamingResponse(c, req.Model, providerType, func() (io.ReadCloser, error) {
			return h.provider.StreamResponses(c.Request().Context(), &req)
		})
	}

	// Non-streaming
	resp, err := h.provider.Responses(c.Request().Context(), &req)
	if err != nil {
		return handleError(c, err)
	}

	// Track usage if enabled
	if h.usageLogger != nil && h.usageLogger.Config().Enabled {
		requestID := c.Response().Header().Get("X-Request-ID")
		usageEntry := usage.ExtractFromResponsesResponse(resp, requestID, "/v1/responses")
		if usageEntry != nil {
			h.usageLogger.Write(usageEntry)
		}
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
