package auditlog

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"
)

// StreamLogWrapper wraps an io.ReadCloser to capture usage data from SSE streams.
// It buffers the last portion of the stream to extract token usage from the
// final SSE event (typically contains usage data in OpenAI-compatible APIs).
type StreamLogWrapper struct {
	io.ReadCloser
	logger    LoggerInterface
	entry     *LogEntry
	buffer    bytes.Buffer
	closed    bool
	startTime time.Time
}

// NewStreamLogWrapper creates a wrapper around a stream to capture usage data.
// When the stream is closed, it parses the final usage data and logs the entry.
func NewStreamLogWrapper(stream io.ReadCloser, logger LoggerInterface, entry *LogEntry) *StreamLogWrapper {
	// Use entry's timestamp as start time for duration calculation
	var startTime time.Time
	if entry != nil {
		startTime = entry.Timestamp
	}
	return &StreamLogWrapper{
		ReadCloser: stream,
		logger:     logger,
		entry:      entry,
		startTime:  startTime,
	}
}

// Read implements io.Reader and buffers recent data to find usage.
func (w *StreamLogWrapper) Read(p []byte) (n int, err error) {
	n, err = w.ReadCloser.Read(p)
	if n > 0 {
		// Buffer recent data to parse final usage event
		w.buffer.Write(p[:n])
		// Keep only last 8KB to find "data: [DONE]" and usage
		if w.buffer.Len() > 8192 {
			// Discard old data, keep recent
			data := w.buffer.Bytes()
			w.buffer.Reset()
			w.buffer.Write(data[len(data)-8192:])
		}
	}
	return n, err
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
	if usage != nil && w.entry != nil && w.entry.Data != nil {
		w.entry.Data.CompletionTokens = usage.CompletionTokens
		w.entry.Data.TotalTokens = usage.TotalTokens
		w.entry.Data.PromptTokens = usage.PromptTokens
	}

	// Write log entry
	if w.logger != nil && w.entry != nil {
		w.logger.Write(w.entry)
	}

	return w.ReadCloser.Close()
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
func WrapStreamForLogging(stream io.ReadCloser, logger LoggerInterface, entry *LogEntry) io.ReadCloser {
	if logger == nil || !logger.Config().Enabled || entry == nil {
		return stream
	}
	return NewStreamLogWrapper(stream, logger, entry)
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
	}

	if baseEntry.Data != nil {
		entryCopy.Data = &LogData{
			RequestID:       baseEntry.Data.RequestID,
			ClientIP:        baseEntry.Data.ClientIP,
			UserAgent:       baseEntry.Data.UserAgent,
			APIKeyHash:      baseEntry.Data.APIKeyHash,
			Method:          baseEntry.Data.Method,
			Path:            baseEntry.Data.Path,
			Stream:          true,
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
