// Package server provides HTTP handlers and server setup for the LLM gateway.
package server

import (
	"errors"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"gomodel/internal/auditlog"
	"gomodel/internal/core"
	"gomodel/internal/guardrails"
	"gomodel/internal/usage"
)

// Handler holds the HTTP handlers
type Handler struct {
	provider    core.RoutableProvider
	logger      auditlog.LoggerInterface
	usageLogger usage.LoggerInterface
	guardrails  *guardrails.Processor
}

// NewHandler creates a new handler with the given routable provider (typically the Router)
func NewHandler(provider core.RoutableProvider, logger auditlog.LoggerInterface, usageLogger usage.LoggerInterface, guardrails *guardrails.Processor) *Handler {
	return &Handler{
		provider:    provider,
		logger:      logger,
		usageLogger: usageLogger,
		guardrails:  guardrails,
	}
}

// handleStreamingResponseWithGuardrails handles SSE streaming responses with optional guardrails de-anonymization.
// It wraps the stream with audit logging, usage tracking, and optionally de-anonymization.
func (h *Handler) handleStreamingResponseWithGuardrails(c echo.Context, model, provider string, guardrailsCtx *guardrails.RequestContext, streamFn func() (io.ReadCloser, error)) error {
	// Call streamFn first - only mark as streaming after success
	// This ensures failed streams are logged normally by handleError/middleware
	stream, err := streamFn()
	if err != nil {
		return handleError(c, err)
	}

	// Mark as streaming so middleware doesn't log (StreamLogWrapper handles it)
	auditlog.MarkEntryAsStreaming(c, true)
	auditlog.EnrichEntryWithStream(c, true)

	// Wrap with guardrails de-anonymization if needed (must be first to see original tokens)
	if h.guardrails != nil {
		stream = h.guardrails.WrapStreamForDeanonymization(stream, guardrailsCtx)
	}

	// Get entry from context and wrap stream for logging
	entry := auditlog.GetStreamEntryFromContext(c)
	streamEntry := auditlog.CreateStreamEntry(entry)
	if streamEntry != nil {
		streamEntry.StatusCode = http.StatusOK // Streaming always starts with 200 OK
	}
	wrappedStream := auditlog.WrapStreamForLogging(stream, h.logger, streamEntry, c.Request().URL.Path)

	// Wrap with usage tracking if enabled
	requestID := c.Request().Header.Get("X-Request-ID")
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

	// Apply guardrails (system prompt injection, anonymization)
	var guardrailsCtx *guardrails.RequestContext
	if h.guardrails != nil {
		processedReq, ctx := h.guardrails.ProcessChatRequest(&req, providerType)
		req = *processedReq
		guardrailsCtx = ctx
	}

	// Create context with request ID for provider
	requestID := c.Request().Header.Get("X-Request-ID")
	ctx := core.WithRequestID(c.Request().Context(), requestID)

	// Handle streaming: proxy the raw SSE stream
	if req.Stream {
		// Enforce returning usage data in streaming responses if configured
		if h.usageLogger != nil && h.usageLogger.Config().EnforceReturningUsageData {
			if req.StreamOptions == nil {
				req.StreamOptions = &core.StreamOptions{}
			}
			req.StreamOptions.IncludeUsage = true
		}
		return h.handleStreamingResponseWithGuardrails(c, req.Model, providerType, guardrailsCtx, func() (io.ReadCloser, error) {
			return h.provider.StreamChatCompletion(ctx, &req)
		})
	}

	// Non-streaming
	resp, err := h.provider.ChatCompletion(ctx, &req)
	if err != nil {
		return handleError(c, err)
	}

	// De-anonymize response if needed
	if h.guardrails != nil {
		resp = h.guardrails.DeanonymizeChatResponse(resp, guardrailsCtx)
	}

	// Track usage if enabled (reuses requestID from context enrichment above)
	if h.usageLogger != nil && h.usageLogger.Config().Enabled {
		usageEntry := usage.ExtractFromChatResponse(resp, requestID, providerType, "/v1/chat/completions")
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
	// Create context with request ID for provider
	requestID := c.Request().Header.Get("X-Request-ID")
	ctx := core.WithRequestID(c.Request().Context(), requestID)

	resp, err := h.provider.ListModels(ctx)
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

	// Apply guardrails (system prompt injection, anonymization)
	var guardrailsCtx *guardrails.RequestContext
	if h.guardrails != nil {
		processedReq, ctx := h.guardrails.ProcessResponsesRequest(&req, providerType)
		req = *processedReq
		guardrailsCtx = ctx
	}

	// Create context with request ID for provider
	requestID := c.Request().Header.Get("X-Request-ID")
	ctx := core.WithRequestID(c.Request().Context(), requestID)

	// Handle streaming: proxy the raw SSE stream
	if req.Stream {
		// Enforce returning usage data in streaming responses if configured
		if h.usageLogger != nil && h.usageLogger.Config().EnforceReturningUsageData {
			if req.StreamOptions == nil {
				req.StreamOptions = &core.StreamOptions{}
			}
			req.StreamOptions.IncludeUsage = true
		}
		return h.handleStreamingResponseWithGuardrails(c, req.Model, providerType, guardrailsCtx, func() (io.ReadCloser, error) {
			return h.provider.StreamResponses(ctx, &req)
		})
	}

	// Non-streaming
	resp, err := h.provider.Responses(ctx, &req)
	if err != nil {
		return handleError(c, err)
	}

	// De-anonymize response if needed
	if h.guardrails != nil {
		resp = h.guardrails.DeanonymizeResponsesResponse(resp, guardrailsCtx)
	}

	// Track usage if enabled (reuses requestID from context enrichment above)
	if h.usageLogger != nil && h.usageLogger.Config().Enabled {
		usageEntry := usage.ExtractFromResponsesResponse(resp, requestID, providerType, "/v1/responses")
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
