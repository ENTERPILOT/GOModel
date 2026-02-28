// Package server provides HTTP handlers and server setup for the LLM gateway.
package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"gomodel/internal/auditlog"
	"gomodel/internal/core"
	"gomodel/internal/usage"
)

// Handler holds the HTTP handlers
type Handler struct {
	provider        core.RoutableProvider
	logger          auditlog.LoggerInterface
	usageLogger     usage.LoggerInterface
	pricingResolver usage.PricingResolver
}

// NewHandler creates a new handler with the given routable provider (typically the Router)
func NewHandler(provider core.RoutableProvider, logger auditlog.LoggerInterface, usageLogger usage.LoggerInterface, pricingResolver usage.PricingResolver) *Handler {
	return &Handler{
		provider:        provider,
		logger:          logger,
		usageLogger:     usageLogger,
		pricingResolver: pricingResolver,
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
	requestID := c.Request().Header.Get("X-Request-ID")
	endpoint := c.Request().URL.Path
	wrappedStream = usage.WrapStreamForUsage(wrappedStream, h.usageLogger, model, provider, requestID, endpoint, h.pricingResolver)

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
//
// @Summary      Create a chat completion
// @Tags         chat
// @Accept       json
// @Produce      json text/event-stream
// @Security     BearerAuth
// @Param        request  body      core.ChatRequest  true  "Chat completion request"
// @Success      200      {object}  core.ChatResponse  "JSON response or SSE stream when stream=true"
// @Failure      400      {object}  core.GatewayError
// @Failure      401      {object}  core.GatewayError
// @Failure      429      {object}  core.GatewayError
// @Failure      502      {object}  core.GatewayError
// @Router       /v1/chat/completions [post]
func (h *Handler) ChatCompletion(c echo.Context) error {
	var req core.ChatRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}

	ctx, providerType := ModelCtx(c)
	requestID := c.Request().Header.Get("X-Request-ID")
	ctx = core.WithRequestID(ctx, requestID)

	if req.Stream {
		if h.usageLogger != nil && h.usageLogger.Config().EnforceReturningUsageData {
			if req.StreamOptions == nil {
				req.StreamOptions = &core.StreamOptions{}
			}
			req.StreamOptions.IncludeUsage = true
		}
		return h.handleStreamingResponse(c, req.Model, providerType, func() (io.ReadCloser, error) {
			return h.provider.StreamChatCompletion(ctx, &req)
		})
	}

	resp, err := h.provider.ChatCompletion(ctx, &req)
	if err != nil {
		return handleError(c, err)
	}

	if h.usageLogger != nil && h.usageLogger.Config().Enabled {
		var pricing *core.ModelPricing
		if h.pricingResolver != nil {
			pricing = h.pricingResolver.ResolvePricing(resp.Model, providerType)
		}
		usageEntry := usage.ExtractFromChatResponse(resp, requestID, providerType, "/v1/chat/completions", pricing)
		if usageEntry != nil {
			h.usageLogger.Write(usageEntry)
		}
	}

	return c.JSON(http.StatusOK, resp)
}

// Health handles GET /health
//
// @Summary      Health check
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /health [get]
func (h *Handler) Health(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ListModels handles GET /v1/models
//
// @Summary      List available models
// @Tags         models
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  core.ModelsResponse
// @Failure      401  {object}  core.GatewayError
// @Failure      502  {object}  core.GatewayError
// @Router       /v1/models [get]
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
//
// @Summary      Create a model response (Responses API)
// @Tags         responses
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      core.ResponsesRequest  true  "Responses API request"
// @Success      200      {object}  core.ResponsesResponse
// @Failure      400      {object}  core.GatewayError
// @Failure      401      {object}  core.GatewayError
// @Failure      429      {object}  core.GatewayError
// @Failure      502      {object}  core.GatewayError
// @Router       /v1/responses [post]
func (h *Handler) Responses(c echo.Context) error {
	var req core.ResponsesRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}

	ctx, providerType := ModelCtx(c)
	requestID := c.Request().Header.Get("X-Request-ID")

	if req.Stream {
		return h.handleStreamingResponse(c, req.Model, providerType, func() (io.ReadCloser, error) {
			return h.provider.StreamResponses(ctx, &req)
		})
	}

	resp, err := h.provider.Responses(ctx, &req)
	if err != nil {
		return handleError(c, err)
	}

	if h.usageLogger != nil && h.usageLogger.Config().Enabled {
		var pricing *core.ModelPricing
		if h.pricingResolver != nil {
			pricing = h.pricingResolver.ResolvePricing(resp.Model, providerType)
		}
		usageEntry := usage.ExtractFromResponsesResponse(resp, requestID, providerType, "/v1/responses", pricing)
		if usageEntry != nil {
			h.usageLogger.Write(usageEntry)
		}
	}

	return c.JSON(http.StatusOK, resp)
}

// Embeddings handles POST /v1/embeddings
func (h *Handler) Embeddings(c echo.Context) error {
	var req core.EmbeddingRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}

	ctx, providerType := ModelCtx(c)
	requestID := c.Request().Header.Get("X-Request-ID")

	resp, err := h.provider.Embeddings(ctx, &req)
	if err != nil {
		return handleError(c, err)
	}

	if h.usageLogger != nil && h.usageLogger.Config().Enabled {
		var pricing *core.ModelPricing
		if h.pricingResolver != nil {
			pricing = h.pricingResolver.ResolvePricing(resp.Model, providerType)
		}
		usageEntry := usage.ExtractFromEmbeddingResponse(resp, requestID, providerType, "/v1/embeddings", pricing)
		if usageEntry != nil {
			h.usageLogger.Write(usageEntry)
		}
	}

	return c.JSON(http.StatusOK, resp)
}

// Batches handles POST /v1/batches.
//
// OpenAI-compatible fields are accepted (`endpoint`, `completion_window`, `metadata`),
// and this gateway also supports inline `requests` for immediate execution.
//
// @Summary      Create and execute a batch
// @Tags         batch
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      core.BatchRequest  true  "Batch request"
// @Success      200      {object}  core.BatchResponse
// @Failure      400      {object}  core.GatewayError
// @Failure      401      {object}  core.GatewayError
// @Failure      502      {object}  core.GatewayError
// @Router       /v1/batches [post]
func (h *Handler) Batches(c echo.Context) error {
	var req core.BatchRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}

	if len(req.Requests) == 0 {
		msg := "requests is required and must not be empty"
		if strings.TrimSpace(req.InputFileID) != "" {
			msg = "input_file_id based batches are not supported yet; provide inline requests"
		}
		return handleError(c, core.NewInvalidRequestError(msg, nil))
	}

	requestID := c.Request().Header.Get("X-Request-ID")
	ctx := core.WithRequestID(c.Request().Context(), requestID)

	results := make([]core.BatchResultItem, 0, len(req.Requests))
	usageSummary := core.BatchUsageSummary{}
	requestCounts := core.BatchRequestCounts{
		Total: len(req.Requests),
	}

	modelSet := map[string]struct{}{}
	providerSet := map[string]struct{}{}

	for i, item := range req.Requests {
		result := core.BatchResultItem{
			Index:    i,
			CustomID: item.CustomID,
			URL:      item.URL,
		}

		endpoint := normalizeBatchEndpoint(resolveBatchEndpoint(req.Endpoint, item.URL))
		if endpoint == "" {
			result.StatusCode = http.StatusBadRequest
			result.Error = &core.BatchError{
				Type:    string(core.ErrorTypeInvalidRequest),
				Message: "batch item url is required (or set top-level endpoint)",
			}
			requestCounts.Failed++
			results = append(results, result)
			continue
		}
		result.URL = endpoint

		method := strings.ToUpper(strings.TrimSpace(item.Method))
		if method == "" {
			method = http.MethodPost
		}
		if method != http.MethodPost {
			result.StatusCode = http.StatusBadRequest
			result.Error = &core.BatchError{
				Type:    string(core.ErrorTypeInvalidRequest),
				Message: "only POST is supported for batch items",
			}
			requestCounts.Failed++
			results = append(results, result)
			continue
		}

		if len(item.Body) == 0 {
			result.StatusCode = http.StatusBadRequest
			result.Error = &core.BatchError{
				Type:    string(core.ErrorTypeInvalidRequest),
				Message: "batch item body is required",
			}
			requestCounts.Failed++
			results = append(results, result)
			continue
		}

		switch endpoint {
		case "/v1/chat/completions":
			var chatReq core.ChatRequest
			if err := json.Unmarshal(item.Body, &chatReq); err != nil {
				result.StatusCode = http.StatusBadRequest
				result.Error = &core.BatchError{
					Type:    string(core.ErrorTypeInvalidRequest),
					Message: "invalid chat request body: " + err.Error(),
				}
				requestCounts.Failed++
				results = append(results, result)
				continue
			}
			if chatReq.Model == "" {
				result.StatusCode = http.StatusBadRequest
				result.Error = &core.BatchError{
					Type:    string(core.ErrorTypeInvalidRequest),
					Message: "model is required",
				}
				requestCounts.Failed++
				results = append(results, result)
				continue
			}
			if !h.provider.Supports(chatReq.Model) {
				result.StatusCode = http.StatusBadRequest
				result.Error = &core.BatchError{
					Type:    string(core.ErrorTypeInvalidRequest),
					Message: "unsupported model: " + chatReq.Model,
				}
				requestCounts.Failed++
				results = append(results, result)
				continue
			}

			chatReq.Stream = false
			providerType := h.provider.GetProviderType(chatReq.Model)
			resp, err := h.provider.ChatCompletion(ctx, &chatReq)
			if err != nil {
				statusCode, batchErr := batchErrorFromErr(err)
				result.StatusCode = statusCode
				result.Error = batchErr
				requestCounts.Failed++
				results = append(results, result)
				continue
			}
			if resp != nil {
				if resp.Model == "" {
					resp.Model = chatReq.Model
				}
				result.Model = resp.Model
				modelSet[result.Model] = struct{}{}
			}
			result.Provider = providerType
			if providerType != "" {
				providerSet[providerType] = struct{}{}
			}
			result.StatusCode = http.StatusOK
			result.Response = resp
			requestCounts.Completed++

			if h.usageLogger != nil && h.usageLogger.Config().Enabled && resp != nil {
				var pricing *core.ModelPricing
				if h.pricingResolver != nil {
					pricing = h.pricingResolver.ResolvePricing(resp.Model, providerType)
				}
				usageEntry := usage.ExtractFromChatResponse(resp, requestID, providerType, "/v1/batches", pricing)
				if usageEntry != nil {
					usageEntry.RawData = ensureBatchEndpointRawData(usageEntry.RawData, endpoint)
					h.usageLogger.Write(usageEntry)
					mergeBatchUsage(&usageSummary, usageEntry)
				}
			}

		case "/v1/responses":
			var responsesReq core.ResponsesRequest
			if err := json.Unmarshal(item.Body, &responsesReq); err != nil {
				result.StatusCode = http.StatusBadRequest
				result.Error = &core.BatchError{
					Type:    string(core.ErrorTypeInvalidRequest),
					Message: "invalid responses request body: " + err.Error(),
				}
				requestCounts.Failed++
				results = append(results, result)
				continue
			}
			if responsesReq.Model == "" {
				result.StatusCode = http.StatusBadRequest
				result.Error = &core.BatchError{
					Type:    string(core.ErrorTypeInvalidRequest),
					Message: "model is required",
				}
				requestCounts.Failed++
				results = append(results, result)
				continue
			}
			if !h.provider.Supports(responsesReq.Model) {
				result.StatusCode = http.StatusBadRequest
				result.Error = &core.BatchError{
					Type:    string(core.ErrorTypeInvalidRequest),
					Message: "unsupported model: " + responsesReq.Model,
				}
				requestCounts.Failed++
				results = append(results, result)
				continue
			}

			responsesReq.Stream = false
			providerType := h.provider.GetProviderType(responsesReq.Model)
			resp, err := h.provider.Responses(ctx, &responsesReq)
			if err != nil {
				statusCode, batchErr := batchErrorFromErr(err)
				result.StatusCode = statusCode
				result.Error = batchErr
				requestCounts.Failed++
				results = append(results, result)
				continue
			}
			if resp != nil {
				if resp.Model == "" {
					resp.Model = responsesReq.Model
				}
				result.Model = resp.Model
				modelSet[result.Model] = struct{}{}
			}
			result.Provider = providerType
			if providerType != "" {
				providerSet[providerType] = struct{}{}
			}
			result.StatusCode = http.StatusOK
			result.Response = resp
			requestCounts.Completed++

			if h.usageLogger != nil && h.usageLogger.Config().Enabled && resp != nil {
				var pricing *core.ModelPricing
				if h.pricingResolver != nil {
					pricing = h.pricingResolver.ResolvePricing(resp.Model, providerType)
				}
				usageEntry := usage.ExtractFromResponsesResponse(resp, requestID, providerType, "/v1/batches", pricing)
				if usageEntry != nil {
					usageEntry.RawData = ensureBatchEndpointRawData(usageEntry.RawData, endpoint)
					h.usageLogger.Write(usageEntry)
					mergeBatchUsage(&usageSummary, usageEntry)
				}
			}

		case "/v1/embeddings":
			var embeddingsReq core.EmbeddingRequest
			if err := json.Unmarshal(item.Body, &embeddingsReq); err != nil {
				result.StatusCode = http.StatusBadRequest
				result.Error = &core.BatchError{
					Type:    string(core.ErrorTypeInvalidRequest),
					Message: "invalid embeddings request body: " + err.Error(),
				}
				requestCounts.Failed++
				results = append(results, result)
				continue
			}
			if embeddingsReq.Model == "" {
				result.StatusCode = http.StatusBadRequest
				result.Error = &core.BatchError{
					Type:    string(core.ErrorTypeInvalidRequest),
					Message: "model is required",
				}
				requestCounts.Failed++
				results = append(results, result)
				continue
			}
			if !h.provider.Supports(embeddingsReq.Model) {
				result.StatusCode = http.StatusBadRequest
				result.Error = &core.BatchError{
					Type:    string(core.ErrorTypeInvalidRequest),
					Message: "unsupported model: " + embeddingsReq.Model,
				}
				requestCounts.Failed++
				results = append(results, result)
				continue
			}

			providerType := h.provider.GetProviderType(embeddingsReq.Model)
			resp, err := h.provider.Embeddings(ctx, &embeddingsReq)
			if err != nil {
				statusCode, batchErr := batchErrorFromErr(err)
				result.StatusCode = statusCode
				result.Error = batchErr
				requestCounts.Failed++
				results = append(results, result)
				continue
			}
			if resp != nil {
				if resp.Model == "" {
					resp.Model = embeddingsReq.Model
				}
				result.Model = resp.Model
				modelSet[result.Model] = struct{}{}
			}
			result.Provider = providerType
			if providerType != "" {
				providerSet[providerType] = struct{}{}
			}
			result.StatusCode = http.StatusOK
			result.Response = resp
			requestCounts.Completed++

			if h.usageLogger != nil && h.usageLogger.Config().Enabled && resp != nil {
				var pricing *core.ModelPricing
				if h.pricingResolver != nil {
					pricing = h.pricingResolver.ResolvePricing(resp.Model, providerType)
				}
				usageEntry := usage.ExtractFromEmbeddingResponse(resp, requestID, providerType, "/v1/batches", pricing)
				if usageEntry != nil {
					usageEntry.RawData = ensureBatchEndpointRawData(usageEntry.RawData, endpoint)
					h.usageLogger.Write(usageEntry)
					mergeBatchUsage(&usageSummary, usageEntry)
				}
			}

		default:
			result.StatusCode = http.StatusBadRequest
			result.Error = &core.BatchError{
				Type:    string(core.ErrorTypeInvalidRequest),
				Message: "unsupported batch item url: " + endpoint,
			}
			requestCounts.Failed++
			results = append(results, result)
			continue
		}

		results = append(results, result)
	}

	// Best-effort enrichment for audit log summary fields.
	auditlog.EnrichEntry(c, selectAuditModel(modelSet), selectAuditProvider(providerSet))

	now := time.Now().Unix()
	status := "completed"
	if requestCounts.Completed == 0 {
		status = "failed"
	}

	resp := core.BatchResponse{
		ID:               "batch_" + uuid.NewString(),
		Object:           "batch",
		Endpoint:         normalizeBatchEndpoint(req.Endpoint),
		InputFileID:      req.InputFileID,
		CompletionWindow: req.CompletionWindow,
		Status:           status,
		CreatedAt:        now,
		CompletedAt:      &now,
		RequestCounts:    requestCounts,
		Metadata:         req.Metadata,
		Usage:            usageSummary,
		Results:          results,
	}

	if resp.Endpoint == "" && len(results) > 0 {
		resp.Endpoint = "mixed"
	}
	if resp.CompletionWindow == "" {
		resp.CompletionWindow = "24h"
	}

	return c.JSON(http.StatusOK, resp)
}

func resolveBatchEndpoint(topLevel, itemURL string) string {
	if strings.TrimSpace(itemURL) != "" {
		return itemURL
	}
	return topLevel
}

func normalizeBatchEndpoint(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Path != "" {
		trimmed = parsed.Path
	}
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return "/"
	}
	return trimmed
}

func batchErrorFromErr(err error) (int, *core.BatchError) {
	var gatewayErr *core.GatewayError
	if errors.As(err, &gatewayErr) {
		return gatewayErr.HTTPStatusCode(), &core.BatchError{
			Type:    string(gatewayErr.Type),
			Message: gatewayErr.Message,
		}
	}

	return http.StatusInternalServerError, &core.BatchError{
		Type:    "internal_error",
		Message: "an unexpected error occurred",
	}
}

func mergeBatchUsage(summary *core.BatchUsageSummary, entry *usage.UsageEntry) {
	if summary == nil || entry == nil {
		return
	}

	summary.InputTokens += entry.InputTokens
	summary.OutputTokens += entry.OutputTokens
	summary.TotalTokens += entry.TotalTokens

	addCost := func(dst **float64, src *float64) {
		if src == nil {
			return
		}
		if *dst == nil {
			v := 0.0
			*dst = &v
		}
		**dst += *src
	}
	addCost(&summary.InputCost, entry.InputCost)
	addCost(&summary.OutputCost, entry.OutputCost)
	addCost(&summary.TotalCost, entry.TotalCost)
}

func selectAuditModel(models map[string]struct{}) string {
	if len(models) == 1 {
		for model := range models {
			return model
		}
	}
	return "batch"
}

func selectAuditProvider(providers map[string]struct{}) string {
	if len(providers) == 1 {
		for provider := range providers {
			return provider
		}
	}
	return "mixed"
}

func ensureBatchEndpointRawData(raw map[string]any, endpoint string) map[string]any {
	if endpoint == "" {
		return raw
	}
	if raw == nil {
		raw = make(map[string]any, 1)
	}
	raw["batch_endpoint"] = endpoint
	return raw
}

// handleError converts gateway errors to appropriate HTTP responses
func handleError(c echo.Context, err error) error {
	var gatewayErr *core.GatewayError
	if errors.As(err, &gatewayErr) {
		auditlog.EnrichEntryWithError(c, string(gatewayErr.Type), gatewayErr.Message)
		return c.JSON(gatewayErr.HTTPStatusCode(), gatewayErr.ToJSON())
	}

	// Fallback for unexpected errors
	auditlog.EnrichEntryWithError(c, "internal_error", "an unexpected error occurred")
	return c.JSON(http.StatusInternalServerError, map[string]interface{}{
		"error": map[string]interface{}{
			"type":    "internal_error",
			"message": "an unexpected error occurred",
		},
	})
}
