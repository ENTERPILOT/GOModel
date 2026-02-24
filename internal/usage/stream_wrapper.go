package usage

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"gomodel/internal/core"
)

// StreamUsageWrapper wraps an io.ReadCloser to capture usage data from SSE streams.
// It buffers the last portion of the stream to extract token usage from the
// final SSE event (typically contains usage data in OpenAI-compatible APIs).
type StreamUsageWrapper struct {
	io.ReadCloser
	logger          LoggerInterface
	pricingResolver PricingResolver
	buffer          bytes.Buffer // rolling buffer for usage extraction
	model           string
	provider        string
	requestID       string
	endpoint        string
	closed          bool
}

// NewStreamUsageWrapper creates a wrapper around a stream to capture usage data.
// When the stream is closed, it parses the final usage data and logs the entry.
func NewStreamUsageWrapper(stream io.ReadCloser, logger LoggerInterface, model, provider, requestID, endpoint string, pricingResolver PricingResolver) *StreamUsageWrapper {
	return &StreamUsageWrapper{
		ReadCloser:      stream,
		logger:          logger,
		pricingResolver: pricingResolver,
		model:           model,
		provider:        provider,
		requestID:       requestID,
		endpoint:        endpoint,
	}
}

// Read implements io.Reader and buffers recent data to find usage.
func (w *StreamUsageWrapper) Read(p []byte) (n int, err error) {
	n, err = w.ReadCloser.Read(p)
	if n > 0 {
		// Buffer recent data to parse final usage event
		if _, errBuf := w.buffer.Write(p[:n]); errBuf != nil {
			return n, errBuf
		}
		// Keep only last SSEBufferSize bytes to find usage
		if w.buffer.Len() > SSEBufferSize {
			// Discard old data, keep recent
			data := w.buffer.Bytes()
			w.buffer.Reset()
			if _, errBuf := w.buffer.Write(data[len(data)-SSEBufferSize:]); errBuf != nil {
				return n, errBuf
			}
		}
	}
	return n, err
}

// Close implements io.Closer, parses usage data, and logs the entry.
func (w *StreamUsageWrapper) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	// Parse final usage from buffered SSE data
	entry := w.parseUsageFromSSE(w.buffer.Bytes())
	if entry != nil && w.logger != nil {
		w.logger.Write(entry)
	}

	return w.ReadCloser.Close()
}

// parseUsageFromSSE extracts usage data from SSE stream buffer.
// OpenAI and compatible APIs include usage in the final event before [DONE].
func (w *StreamUsageWrapper) parseUsageFromSSE(data []byte) *UsageEntry {
	// Split into SSE events
	events := bytes.Split(data, []byte("\n\n"))

	// Search from the end for usage data
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		// Skip empty events and [DONE]
		if len(event) == 0 || bytes.Contains(event, []byte("[DONE]")) {
			continue
		}

		// Find data line
		lines := bytes.Split(event, []byte("\n"))
		for _, line := range lines {
			if bytes.HasPrefix(line, []byte("data: ")) {
				jsonData := bytes.TrimPrefix(line, []byte("data: "))
				entry := w.extractUsageFromJSON(jsonData)
				if entry != nil {
					return entry
				}
			}
		}
	}

	return nil
}

// extractUsageFromJSON attempts to extract usage from a JSON chunk.
func (w *StreamUsageWrapper) extractUsageFromJSON(data []byte) *UsageEntry {
	// Try to parse as a generic map
	var chunk map[string]interface{}
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil
	}

	// Get provider ID (response ID)
	providerID, _ := chunk["id"].(string)

	// Get model if available in the chunk (may differ from request model)
	model := w.model
	if m, ok := chunk["model"].(string); ok && m != "" {
		model = m
	}

	// Look for usage field (OpenAI/ChatCompletion format)
	usageRaw, ok := chunk["usage"]

	// If not found at top level, check for Responses API format:
	// {"type": "response.done", "response": {"id": "...", "usage": {...}}}
	if !ok {
		if eventType, _ := chunk["type"].(string); eventType == "response.done" {
			if response, respOk := chunk["response"].(map[string]interface{}); respOk {
				usageRaw, ok = response["usage"]
				// Extract provider ID and model from response object
				if id, idOk := response["id"].(string); idOk && id != "" {
					providerID = id
				}
				if m, mOk := response["model"].(string); mOk && m != "" {
					model = m
				}
			}
		}
	}

	if !ok {
		return nil
	}

	usageMap, ok := usageRaw.(map[string]interface{})
	if !ok {
		return nil
	}

	var inputTokens, outputTokens, totalTokens int
	rawData := make(map[string]any)

	// Extract standard fields
	if v, ok := usageMap["prompt_tokens"].(float64); ok {
		inputTokens = int(v)
	}
	if v, ok := usageMap["input_tokens"].(float64); ok {
		inputTokens = int(v)
	}
	if v, ok := usageMap["completion_tokens"].(float64); ok {
		outputTokens = int(v)
	}
	if v, ok := usageMap["output_tokens"].(float64); ok {
		outputTokens = int(v)
	}
	if v, ok := usageMap["total_tokens"].(float64); ok {
		totalTokens = int(v)
	}

	// Extract extended usage data (provider-specific) using the field set
	// derived from providerMappings in cost.go (single source of truth).
	for field := range extendedFieldSet {
		if v, ok := usageMap[field].(float64); ok && v > 0 {
			rawData[field] = int(v)
		}
	}

	// Also check for nested prompt_tokens_details and completion_tokens_details (OpenAI)
	if details, ok := usageMap["prompt_tokens_details"].(map[string]interface{}); ok {
		for k, v := range details {
			if fv, ok := v.(float64); ok && fv > 0 {
				rawData["prompt_"+k] = int(fv)
			}
		}
	}
	if details, ok := usageMap["completion_tokens_details"].(map[string]interface{}); ok {
		for k, v := range details {
			if fv, ok := v.(float64); ok && fv > 0 {
				rawData["completion_"+k] = int(fv)
			}
		}
	}

	// Only create entry if we found some usage data
	if inputTokens > 0 || outputTokens > 0 || totalTokens > 0 {
		if len(rawData) == 0 {
			rawData = nil
		}

		// Resolve pricing for cost calculation
		var pricingArgs []*core.ModelPricing
		if w.pricingResolver != nil {
			if p := w.pricingResolver.ResolvePricing(model, w.provider); p != nil {
				pricingArgs = append(pricingArgs, p)
			}
		}

		return ExtractFromSSEUsage(
			providerID,
			inputTokens, outputTokens, totalTokens,
			rawData,
			w.requestID, model, w.provider, w.endpoint,
			pricingArgs...,
		)
	}

	return nil
}

// WrapStreamForUsage wraps a stream with usage tracking if enabled.
// This is a convenience function for use in handlers.
func WrapStreamForUsage(stream io.ReadCloser, logger LoggerInterface, model, provider, requestID, endpoint string, pricingResolver PricingResolver) io.ReadCloser {
	if logger == nil || !logger.Config().Enabled {
		return stream
	}
	return NewStreamUsageWrapper(stream, logger, model, provider, requestID, endpoint, pricingResolver)
}

// IsModelInteractionPath returns true if the path is an AI model endpoint
func IsModelInteractionPath(path string) bool {
	modelPaths := []string{
		"/v1/chat/completions",
		"/v1/responses",
	}
	for _, p := range modelPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}
