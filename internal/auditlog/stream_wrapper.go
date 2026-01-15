package auditlog

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"
)

// maxContentCapture is the maximum size of accumulated content (1MB)
const maxContentCapture = 1024 * 1024

// streamResponseBuilder accumulates data from SSE events to reconstruct a response
type streamResponseBuilder struct {
	// ChatCompletion fields
	ID           string
	Object       string
	Model        string
	Created      int64
	Role         string
	FinishReason string
	Content      strings.Builder // accumulated delta content

	// Responses API fields
	IsResponsesAPI bool
	ResponseID     string
	CreatedAt      int64
	Status         string

	// Tracking
	contentLen int // track content length to enforce limit
	truncated  bool
}

// StreamLogWrapper wraps an io.ReadCloser to capture usage data from SSE streams.
// It buffers the last portion of the stream to extract token usage from the
// final SSE event (typically contains usage data in OpenAI-compatible APIs).
type StreamLogWrapper struct {
	io.ReadCloser
	logger    LoggerInterface
	entry     *LogEntry
	buffer    bytes.Buffer // rolling 8KB buffer for usage extraction
	builder   *streamResponseBuilder
	logBodies bool
	path      string // request path to detect endpoint type
	closed    bool
	startTime time.Time
	pending   []byte // pending partial SSE data between reads
}

// NewStreamLogWrapper creates a wrapper around a stream to capture usage data.
// When the stream is closed, it parses the final usage data and logs the entry.
// The path parameter is used to detect whether this is a ChatCompletion or Responses API request.
func NewStreamLogWrapper(stream io.ReadCloser, logger LoggerInterface, entry *LogEntry, path string) *StreamLogWrapper {
	// Use entry's timestamp as start time for duration calculation
	var startTime time.Time
	if entry != nil {
		startTime = entry.Timestamp
	}

	// Check if body logging is enabled
	logBodies := false
	if logger != nil {
		logBodies = logger.Config().LogBodies
	}

	// Initialize builder if body logging is enabled
	var builder *streamResponseBuilder
	if logBodies {
		builder = &streamResponseBuilder{
			IsResponsesAPI: strings.HasPrefix(path, "/v1/responses"),
		}
	}

	return &StreamLogWrapper{
		ReadCloser: stream,
		logger:     logger,
		entry:      entry,
		startTime:  startTime,
		logBodies:  logBodies,
		path:       path,
		builder:    builder,
	}
}

// Read implements io.Reader and buffers recent data to find usage.
func (w *StreamLogWrapper) Read(p []byte) (n int, err error) {
	n, err = w.ReadCloser.Read(p)
	if n > 0 {
		// Parse SSE events and accumulate content if body logging is enabled
		if w.logBodies && w.builder != nil {
			w.processSSEData(p[:n])
		}

		// Buffer recent data to parse final usage event
		if _, errBuf := w.buffer.Write(p[:n]); errBuf != nil {
			return n, errBuf
		}
		// Keep only last 8KB to find "data: [DONE]" and usage
		if w.buffer.Len() > 8192 {
			// Discard old data, keep recent
			data := w.buffer.Bytes()
			w.buffer.Reset()
			if _, errBuf := w.buffer.Write(data[len(data)-8192:]); errBuf != nil {
				return n, errBuf
			}
		}
	}
	return n, err
}

// processSSEData parses SSE events from the data chunk and accumulates content
func (w *StreamLogWrapper) processSSEData(data []byte) {
	// Prepend any pending data from previous read
	if len(w.pending) > 0 {
		data = append(w.pending, data...)
		w.pending = nil
	}

	// Split on double newline (SSE event separator)
	for {
		idx := bytes.Index(data, []byte("\n\n"))
		if idx == -1 {
			// No complete event, save as pending
			if len(data) > 0 {
				w.pending = make([]byte, len(data))
				copy(w.pending, data)
			}
			return
		}

		event := data[:idx]
		data = data[idx+2:]

		w.processSSEEvent(event)
	}
}

// processSSEEvent processes a single SSE event
func (w *StreamLogWrapper) processSSEEvent(event []byte) {
	// Find the data line
	lines := bytes.Split(event, []byte("\n"))
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("data: ")) {
			jsonData := bytes.TrimPrefix(line, []byte("data: "))
			// Skip [DONE] marker
			if bytes.Equal(jsonData, []byte("[DONE]")) {
				continue
			}
			w.parseEventJSON(jsonData)
		}
	}
}

// parseEventJSON parses the JSON from an SSE event and accumulates data
func (w *StreamLogWrapper) parseEventJSON(data []byte) {
	var event map[string]interface{}
	if err := json.Unmarshal(data, &event); err != nil {
		return
	}

	if w.builder.IsResponsesAPI {
		w.parseResponsesAPIEvent(event)
	} else {
		w.parseChatCompletionEvent(event)
	}
}

// parseChatCompletionEvent extracts data from a ChatCompletion streaming chunk
func (w *StreamLogWrapper) parseChatCompletionEvent(event map[string]interface{}) {
	// Extract metadata from first event
	if w.builder.ID == "" {
		if id, ok := event["id"].(string); ok {
			w.builder.ID = id
		}
		if obj, ok := event["object"].(string); ok {
			w.builder.Object = obj
		}
		if model, ok := event["model"].(string); ok {
			w.builder.Model = model
		}
		if created, ok := event["created"].(float64); ok {
			w.builder.Created = int64(created)
		}
	}

	// Extract delta content from choices
	if choices, ok := event["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			// Extract finish_reason
			if fr, ok := choice["finish_reason"].(string); ok && fr != "" {
				w.builder.FinishReason = fr
			}

			// Extract delta
			if delta, ok := choice["delta"].(map[string]interface{}); ok {
				// Extract role (usually in first chunk)
				if role, ok := delta["role"].(string); ok {
					w.builder.Role = role
				}
				// Extract and accumulate content
				if content, ok := delta["content"].(string); ok && content != "" {
					if !w.builder.truncated && w.builder.contentLen < maxContentCapture {
						remaining := maxContentCapture - w.builder.contentLen
						if len(content) > remaining {
							content = content[:remaining]
							w.builder.truncated = true
						}
						w.builder.Content.WriteString(content)
						w.builder.contentLen += len(content)
					}
				}
			}
		}
	}
}

// parseResponsesAPIEvent extracts data from a Responses API streaming event
func (w *StreamLogWrapper) parseResponsesAPIEvent(event map[string]interface{}) {
	eventType, _ := event["type"].(string)

	switch eventType {
	case "response.created", "response.done":
		// Extract response metadata
		if resp, ok := event["response"].(map[string]interface{}); ok {
			if id, ok := resp["id"].(string); ok {
				w.builder.ResponseID = id
			}
			if status, ok := resp["status"].(string); ok {
				w.builder.Status = status
			}
			if model, ok := resp["model"].(string); ok {
				w.builder.Model = model
			}
			if createdAt, ok := resp["created_at"].(float64); ok {
				w.builder.CreatedAt = int64(createdAt)
			}
		}

	case "response.output_text.delta":
		// Accumulate text delta
		if delta, ok := event["delta"].(string); ok && delta != "" {
			if !w.builder.truncated && w.builder.contentLen < maxContentCapture {
				remaining := maxContentCapture - w.builder.contentLen
				if len(delta) > remaining {
					delta = delta[:remaining]
					w.builder.truncated = true
				}
				w.builder.Content.WriteString(delta)
				w.builder.contentLen += len(delta)
			}
		}
	}
}

// Close implements io.Closer, parses usage data, and logs the entry.
func (w *StreamLogWrapper) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	// Calculate duration from start time
	if w.entry != nil && !w.startTime.IsZero() {
		w.entry.DurationNs = time.Since(w.startTime).Nanoseconds()
	}

	// Parse final usage from buffered SSE data
	usage := parseUsageFromSSE(w.buffer.Bytes())
	if usage != nil && w.entry != nil {
		w.entry.CompletionTokens = usage.CompletionTokens
		w.entry.TotalTokens = usage.TotalTokens
		w.entry.PromptTokens = usage.PromptTokens
	}

	// Build and store reconstructed response body if enabled
	if w.logBodies && w.builder != nil && w.entry != nil && w.entry.Data != nil {
		if w.builder.IsResponsesAPI {
			w.entry.Data.ResponseBody = w.builder.buildResponsesAPIResponse(usage)
		} else {
			w.entry.Data.ResponseBody = w.builder.buildChatCompletionResponse(usage)
		}
		w.entry.Data.ResponseBodyTooBigToHandle = w.builder.truncated
	}

	// Write log entry
	if w.logger != nil && w.entry != nil {
		w.logger.Write(w.entry)
	}

	return w.ReadCloser.Close()
}

// buildChatCompletionResponse constructs a ChatCompletion response from accumulated data
func (b *streamResponseBuilder) buildChatCompletionResponse(usage *Usage) map[string]interface{} {
	response := map[string]interface{}{
		"id":      b.ID,
		"object":  "chat.completion",
		"model":   b.Model,
		"created": b.Created,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    b.Role,
					"content": b.Content.String(),
				},
				"finish_reason": b.FinishReason,
			},
		},
	}

	if usage != nil {
		response["usage"] = map[string]interface{}{
			"prompt_tokens":     usage.PromptTokens,
			"completion_tokens": usage.CompletionTokens,
			"total_tokens":      usage.TotalTokens,
		}
	}

	return response
}

// buildResponsesAPIResponse constructs a Responses API response from accumulated data
func (b *streamResponseBuilder) buildResponsesAPIResponse(usage *Usage) map[string]interface{} {
	response := map[string]interface{}{
		"id":         b.ResponseID,
		"object":     "response",
		"model":      b.Model,
		"created_at": b.CreatedAt,
		"status":     b.Status,
		"output": []map[string]interface{}{
			{
				"type": "message",
				"role": "assistant",
				"content": []map[string]interface{}{
					{
						"type": "output_text",
						"text": b.Content.String(),
					},
				},
			},
		},
	}

	if usage != nil {
		response["usage"] = map[string]interface{}{
			"input_tokens":  usage.PromptTokens,
			"output_tokens": usage.CompletionTokens,
			"total_tokens":  usage.TotalTokens,
		}
	}

	return response
}

// parseUsageFromSSE extracts usage data from SSE stream buffer.
// OpenAI and compatible APIs include usage in the final event before [DONE].
func parseUsageFromSSE(data []byte) *Usage {
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
				usage := extractUsageFromJSON(jsonData)
				if usage != nil {
					return usage
				}
			}
		}
	}

	return nil
}

// extractUsageFromJSON attempts to extract usage from a JSON chunk.
func extractUsageFromJSON(data []byte) *Usage {
	// Try to parse as a generic map
	var chunk map[string]interface{}
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil
	}

	// Look for usage field (OpenAI format)
	usageRaw, ok := chunk["usage"]
	if !ok {
		return nil
	}

	usageMap, ok := usageRaw.(map[string]interface{})
	if !ok {
		return nil
	}

	usage := &Usage{}

	if v, ok := usageMap["prompt_tokens"].(float64); ok {
		usage.PromptTokens = int(v)
	}
	if v, ok := usageMap["completion_tokens"].(float64); ok {
		usage.CompletionTokens = int(v)
	}
	if v, ok := usageMap["total_tokens"].(float64); ok {
		usage.TotalTokens = int(v)
	}

	// Only return if we found some usage data
	if usage.PromptTokens > 0 || usage.CompletionTokens > 0 || usage.TotalTokens > 0 {
		return usage
	}

	return nil
}

// WrapStreamForLogging wraps a stream with logging if enabled.
// This is a convenience function for use in handlers.
// The path parameter is used to detect whether this is a ChatCompletion or Responses API request.
func WrapStreamForLogging(stream io.ReadCloser, logger LoggerInterface, entry *LogEntry, path string) io.ReadCloser {
	if logger == nil || !logger.Config().Enabled || entry == nil {
		return stream
	}
	return NewStreamLogWrapper(stream, logger, entry, path)
}

// CreateStreamEntry creates a new log entry for a streaming request.
// This should be called before starting the stream.
func CreateStreamEntry(baseEntry *LogEntry) *LogEntry {
	if baseEntry == nil {
		return nil
	}

	// Create a copy of the entry for the stream
	// The stream wrapper will complete and write it when the stream closes
	entryCopy := &LogEntry{
		ID:         baseEntry.ID,
		Timestamp:  baseEntry.Timestamp,
		DurationNs: baseEntry.DurationNs,
		Model:      baseEntry.Model,
		Provider:   baseEntry.Provider,
		StatusCode: baseEntry.StatusCode,
		// Copy extracted fields
		RequestID: baseEntry.RequestID,
		ClientIP:  baseEntry.ClientIP,
		Method:    baseEntry.Method,
		Path:      baseEntry.Path,
		Stream:    true, // Mark as streaming
	}

	if baseEntry.Data != nil {
		entryCopy.Data = &LogData{
			UserAgent:       baseEntry.Data.UserAgent,
			APIKeyHash:      baseEntry.Data.APIKeyHash,
			Temperature:     baseEntry.Data.Temperature,
			MaxTokens:       baseEntry.Data.MaxTokens,
			RequestHeaders:  copyMap(baseEntry.Data.RequestHeaders),
			ResponseHeaders: copyMap(baseEntry.Data.ResponseHeaders),
			RequestBody:     baseEntry.Data.RequestBody,
		}
	}

	return entryCopy
}

// copyMap creates a shallow copy of a string map
func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// GetStreamEntryFromContext retrieves the log entry from Echo context for streaming.
// This allows handlers to get the entry for wrapping streams.
func GetStreamEntryFromContext(c interface{ Get(string) interface{} }) *LogEntry {
	entryVal := c.Get(string(LogEntryKey))
	if entryVal == nil {
		return nil
	}

	entry, ok := entryVal.(*LogEntry)
	if !ok {
		return nil
	}

	return entry
}

// MarkEntryAsStreaming marks the entry as a streaming request so the middleware
// knows not to log it (the stream wrapper will handle logging).
func MarkEntryAsStreaming(c interface{ Set(string, interface{}) }, isStreaming bool) {
	// We use a simple marker in the context
	c.Set(string(LogEntryKey)+"_streaming", isStreaming)
}

// IsEntryMarkedAsStreaming checks if the entry is marked as streaming.
func IsEntryMarkedAsStreaming(c interface{ Get(string) interface{} }) bool {
	val := c.Get(string(LogEntryKey) + "_streaming")
	if val == nil {
		return false
	}
	streaming, _ := val.(bool)
	return streaming
}

// IsModelInteractionPath returns true if the path is an AI model endpoint
func IsModelInteractionPath(path string) bool {
	modelPaths := []string{
		"/v1/chat/completions",
		"/v1/responses",
		"/v1/models",
	}
	for _, p := range modelPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}
