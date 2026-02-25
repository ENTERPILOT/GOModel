package usage

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"gomodel/internal/core"
)

// maxEventBufferRemainder is a safety valve for the event buffer remainder.
// If an incomplete event exceeds this size, it's discarded to prevent unbounded memory growth.
const maxEventBufferRemainder = 256 * 1024 // 256KB

// StreamUsageWrapper wraps an io.ReadCloser to capture usage data from SSE streams.
// It incrementally parses SSE events as they arrive (on each \n\n boundary),
// extracting and caching usage data immediately when found. This handles
// arbitrarily large events like the Responses API's response.completed which includes
// the full response object alongside usage data.
type StreamUsageWrapper struct {
	io.ReadCloser
	logger          LoggerInterface
	pricingResolver PricingResolver
	eventBuffer     bytes.Buffer // accumulates raw bytes until \n\n found
	cachedEntry     *UsageEntry  // stores extracted usage from latest event containing it
	model           string
	provider        string
	requestID       string
	endpoint        string
	closed          bool
}

// NewStreamUsageWrapper creates a wrapper around a stream to capture usage data.
// When the stream is closed, it logs the cached usage entry if one was found.
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

// Read implements io.Reader. It reads from the underlying stream and incrementally
// parses complete SSE events to extract usage data as they arrive.
func (w *StreamUsageWrapper) Read(p []byte) (n int, err error) {
	n, err = w.ReadCloser.Read(p)
	if n > 0 {
		w.eventBuffer.Write(p[:n])
		w.processCompleteEvents()
	}
	return n, err
}

// processCompleteEvents scans the event buffer for complete SSE events (delimited by \n\n),
// extracts usage from each, and keeps only the unprocessed remainder.
func (w *StreamUsageWrapper) processCompleteEvents() {
	data := w.eventBuffer.Bytes()

	// Find the last complete event boundary
	lastBoundary := bytes.LastIndex(data, []byte("\n\n"))
	if lastBoundary < 0 {
		// No complete event yet — apply safety valve on remainder size.
		// Trim to keep only the newest bytes so partial events are preserved.
		if w.eventBuffer.Len() > maxEventBufferRemainder {
			tail := w.eventBuffer.Bytes()
			start := len(tail) - maxEventBufferRemainder
			// If the underlying capacity has grown too large, allocate a new buffer,
			// otherwise reuse the existing backing array to avoid unnecessary allocs.
			if cap(tail) > maxEventBufferRemainder*2 {
				var newBuf bytes.Buffer
				newBuf.Write(tail[start:])
				w.eventBuffer = newBuf
			} else {
				copy(tail[:maxEventBufferRemainder], tail[start:])
				w.eventBuffer.Reset()
				w.eventBuffer.Write(tail[:maxEventBufferRemainder])
			}
		}
		return
	}

	// Split into complete events and remainder
	completeData := data[:lastBoundary]
	remainder := data[lastBoundary+2:] // skip the \n\n

	// Process each complete event
	events := bytes.Split(completeData, []byte("\n\n"))
	for _, event := range events {
		if len(event) == 0 || bytes.Contains(event, []byte("[DONE]")) {
			continue
		}

		// Find data line(s) in this event
		lines := bytes.Split(event, []byte("\n"))
		for _, line := range lines {
			trimmed := line
			// Skip "event:" lines
			if bytes.HasPrefix(trimmed, []byte("event:")) {
				continue
			}
			if bytes.HasPrefix(trimmed, []byte("data: ")) {
				jsonData := bytes.TrimPrefix(trimmed, []byte("data: "))
				entry := w.extractUsageFromJSON(jsonData)
				if entry != nil {
					w.cachedEntry = entry
				}
			}
		}
	}

	// Keep only the remainder, preventing capacity leaks from oversized buffers.
	// Write copies remainder into a fresh buffer, releasing the old oversized backing array.
	if len(remainder) > maxEventBufferRemainder {
		remainder = remainder[len(remainder)-maxEventBufferRemainder:]
	}
	if w.eventBuffer.Cap() > maxEventBufferRemainder*2 {
		var newBuf bytes.Buffer
		newBuf.Write(remainder)
		w.eventBuffer = newBuf
	} else {
		w.eventBuffer.Reset()
		w.eventBuffer.Write(remainder)
	}
}

// Close implements io.Closer. It processes any remaining buffer data,
// logs the cached usage entry if found, and closes the underlying stream.
func (w *StreamUsageWrapper) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	// Process any remaining data in the buffer as a final attempt
	if w.eventBuffer.Len() > 0 {
		entry := w.parseRemainingBuffer(w.eventBuffer.Bytes())
		if entry != nil {
			w.cachedEntry = entry
		}
	}

	// Log the cached entry
	if w.cachedEntry != nil && w.logger != nil {
		w.logger.Write(w.cachedEntry)
	}

	return w.ReadCloser.Close()
}

// parseRemainingBuffer is a fallback parser for any unterminated data left in the buffer
// at Close time. It splits on \n\n and searches for usage data.
func (w *StreamUsageWrapper) parseRemainingBuffer(data []byte) *UsageEntry {
	events := bytes.Split(data, []byte("\n\n"))

	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if len(event) == 0 || bytes.Contains(event, []byte("[DONE]")) {
			continue
		}

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
	// {"type": "response.completed", "response": {"id": "...", "usage": {...}}}
	if !ok {
		if eventType, _ := chunk["type"].(string); eventType == "response.completed" || eventType == "response.done" {
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
