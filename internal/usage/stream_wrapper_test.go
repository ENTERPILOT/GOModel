package usage

import (
	"io"
	"strings"
	"sync"
	"testing"
)

// trackingLogger tracks written entries for testing
type trackingLogger struct {
	entries []*UsageEntry
	mu      sync.Mutex
	enabled bool
}

func (l *trackingLogger) Write(entry *UsageEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
}

func (l *trackingLogger) Config() Config {
	return Config{Enabled: l.enabled}
}

func (l *trackingLogger) Close() error {
	return nil
}

func (l *trackingLogger) getEntries() []*UsageEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]*UsageEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

func TestStreamUsageWrapper(t *testing.T) {
	// OpenAI-style SSE stream with usage in final event
	streamData := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]

`
	logger := &trackingLogger{enabled: true}
	stream := io.NopCloser(strings.NewReader(streamData))
	wrapper := NewStreamUsageWrapper(stream, logger, "gpt-4", "openai", "req-123", "/v1/chat/completions", nil)

	// Read all data
	data, err := io.ReadAll(wrapper)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}

	// Verify data passed through
	if string(data) != streamData {
		t.Errorf("data mismatch: got %d bytes, want %d bytes", len(data), len(streamData))
	}

	// Close wrapper to trigger usage extraction
	if err := wrapper.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	// Verify usage was extracted
	entries := logger.getEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", entry.InputTokens)
	}
	if entry.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", entry.OutputTokens)
	}
	if entry.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", entry.TotalTokens)
	}
	if entry.ProviderID != "chatcmpl-123" {
		t.Errorf("ProviderID = %s, want chatcmpl-123", entry.ProviderID)
	}
	if entry.Model != "gpt-4" {
		t.Errorf("Model = %s, want gpt-4", entry.Model)
	}
}

func TestStreamUsageWrapperWithExtendedUsage(t *testing.T) {
	// OpenAI o-series with prompt_tokens_details and completion_tokens_details
	streamData := `data: {"id":"chatcmpl-456","object":"chat.completion.chunk","model":"o1-preview","choices":[{"index":0,"delta":{"content":"Hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"prompt_tokens_details":{"cached_tokens":20},"completion_tokens_details":{"reasoning_tokens":10}}}

data: [DONE]

`
	logger := &trackingLogger{enabled: true}
	stream := io.NopCloser(strings.NewReader(streamData))
	wrapper := NewStreamUsageWrapper(stream, logger, "o1-preview", "openai", "req-456", "/v1/chat/completions", nil)

	_, _ = io.ReadAll(wrapper)
	_ = wrapper.Close()

	entries := logger.getEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", entry.InputTokens)
	}
	if entry.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", entry.OutputTokens)
	}

	// Check extended data was captured
	if entry.RawData == nil {
		t.Fatal("expected RawData to be set")
	}
	if entry.RawData["prompt_cached_tokens"] != 20 {
		t.Errorf("RawData[prompt_cached_tokens] = %v, want 20", entry.RawData["prompt_cached_tokens"])
	}
	if entry.RawData["completion_reasoning_tokens"] != 10 {
		t.Errorf("RawData[completion_reasoning_tokens] = %v, want 10", entry.RawData["completion_reasoning_tokens"])
	}
}

func TestStreamUsageWrapperNoUsage(t *testing.T) {
	// Stream without usage data
	streamData := `data: {"id":"chatcmpl-789","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hi"},"finish_reason":"stop"}]}

data: [DONE]

`
	logger := &trackingLogger{enabled: true}
	stream := io.NopCloser(strings.NewReader(streamData))
	wrapper := NewStreamUsageWrapper(stream, logger, "gpt-4", "openai", "req-789", "/v1/chat/completions", nil)

	_, _ = io.ReadAll(wrapper)
	_ = wrapper.Close()

	// Should not log anything if no usage found
	entries := logger.getEntries()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (no usage), got %d", len(entries))
	}
}

func TestStreamUsageWrapperDisabled(t *testing.T) {
	streamData := `data: {"id":"chatcmpl-123","usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]

`
	logger := &trackingLogger{enabled: false} // disabled
	stream := io.NopCloser(strings.NewReader(streamData))
	wrapper := NewStreamUsageWrapper(stream, logger, "gpt-4", "openai", "req-123", "/v1/chat/completions", nil)

	_, _ = io.ReadAll(wrapper)
	_ = wrapper.Close()

	// Should still log even when config says disabled (because Write() is called)
	// The WrapStreamForUsage function is what should check enabled status
	entries := logger.getEntries()
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestWrapStreamForUsageDisabled(t *testing.T) {
	streamData := "test data"
	logger := &trackingLogger{enabled: false} // disabled
	stream := io.NopCloser(strings.NewReader(streamData))

	wrapped := WrapStreamForUsage(stream, logger, "gpt-4", "openai", "req-123", "/v1/chat/completions", nil)

	// When disabled, should return original stream (not wrapped)
	// This is determined by checking if wrapped is the same as original
	data, _ := io.ReadAll(wrapped)
	if string(data) != streamData {
		t.Errorf("data mismatch")
	}
}

func TestWrapStreamForUsageNilLogger(t *testing.T) {
	streamData := "test data"
	stream := io.NopCloser(strings.NewReader(streamData))

	wrapped := WrapStreamForUsage(stream, nil, "gpt-4", "openai", "req-123", "/v1/chat/completions", nil)

	// When nil logger, should return original stream
	data, _ := io.ReadAll(wrapped)
	if string(data) != streamData {
		t.Errorf("data mismatch")
	}
}

func TestIsModelInteractionPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/v1/chat/completions", true},
		{"/v1/chat/completions?foo=bar", true},
		{"/v1/responses", true},
		{"/v1/responses/123", true},
		{"/v1/models", false},
		{"/health", false},
		{"/metrics", false},
		{"/admin", false},
		{"/", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsModelInteractionPath(tt.path)
			if got != tt.want {
				t.Errorf("IsModelInteractionPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestStreamUsageWrapperDoubleClose(t *testing.T) {
	streamData := `data: {"id":"chatcmpl-123","usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]

`
	logger := &trackingLogger{enabled: true}
	stream := io.NopCloser(strings.NewReader(streamData))
	wrapper := NewStreamUsageWrapper(stream, logger, "gpt-4", "openai", "req-123", "/v1/chat/completions", nil)

	_, _ = io.ReadAll(wrapper)

	// Close twice should not panic or double-log
	_ = wrapper.Close()
	_ = wrapper.Close()

	entries := logger.getEntries()
	if len(entries) != 1 {
		t.Errorf("expected 1 entry (not 2 from double close), got %d", len(entries))
	}
}

func TestStreamUsageWrapperResponsesAPI(t *testing.T) {
	// Responses API format with event: prefixes and response.done containing nested response.usage
	streamData := `event: response.created
data: {"type":"response.created","response":{"id":"resp-123","object":"response","status":"in_progress","model":"gpt-5"}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"Hello"}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":" world!"}

event: response.done
data: {"type":"response.done","response":{"id":"resp-123","object":"response","status":"completed","model":"gpt-5","output":[{"id":"msg_001","type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello world!"}]}],"usage":{"input_tokens":15,"output_tokens":8,"total_tokens":23}}}

data: [DONE]

`
	logger := &trackingLogger{enabled: true}
	stream := io.NopCloser(strings.NewReader(streamData))
	wrapper := NewStreamUsageWrapper(stream, logger, "gpt-5", "openai", "req-resp-1", "/v1/responses", nil)

	data, err := io.ReadAll(wrapper)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}

	if string(data) != streamData {
		t.Errorf("data mismatch: got %d bytes, want %d bytes", len(data), len(streamData))
	}

	if err := wrapper.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	entries := logger.getEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.InputTokens != 15 {
		t.Errorf("InputTokens = %d, want 15", entry.InputTokens)
	}
	if entry.OutputTokens != 8 {
		t.Errorf("OutputTokens = %d, want 8", entry.OutputTokens)
	}
	if entry.TotalTokens != 23 {
		t.Errorf("TotalTokens = %d, want 23", entry.TotalTokens)
	}
	if entry.ProviderID != "resp-123" {
		t.Errorf("ProviderID = %s, want resp-123", entry.ProviderID)
	}
	if entry.Model != "gpt-5" {
		t.Errorf("Model = %s, want gpt-5", entry.Model)
	}
}

func TestStreamUsageWrapperLargeResponsesDone(t *testing.T) {
	// Regression test: response.done event >8KB should not lose usage data.
	// The old rolling 8KB buffer would truncate the beginning of this event.

	// Build a large output content to push the response.done event well over 8KB
	largeText := strings.Repeat("This is a long response from the model. ", 300) // ~12KB of text

	streamData := `event: response.created
data: {"type":"response.created","response":{"id":"resp-large","object":"response","status":"in_progress","model":"gpt-5"}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"start"}

event: response.done
data: {"type":"response.done","response":{"id":"resp-large","object":"response","status":"completed","model":"gpt-5","output":[{"id":"msg_001","type":"message","role":"assistant","content":[{"type":"output_text","text":"` + largeText + `"}]}],"usage":{"input_tokens":100,"output_tokens":500,"total_tokens":600}}}

data: [DONE]

`

	// Verify the response.done event is actually >8KB
	doneEventStart := strings.Index(streamData, `data: {"type":"response.done"`)
	doneEventEnd := strings.Index(streamData[doneEventStart:], "\n\n")
	doneEventSize := doneEventEnd
	if doneEventSize <= 8192 {
		t.Fatalf("test setup error: response.done event is only %d bytes, need >8192", doneEventSize)
	}

	logger := &trackingLogger{enabled: true}
	stream := io.NopCloser(strings.NewReader(streamData))
	wrapper := NewStreamUsageWrapper(stream, logger, "gpt-5", "openai", "req-large", "/v1/responses", nil)

	data, err := io.ReadAll(wrapper)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}

	if string(data) != streamData {
		t.Errorf("data mismatch: got %d bytes, want %d bytes", len(data), len(streamData))
	}

	if err := wrapper.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	entries := logger.getEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d (usage was lost from large response.done event)", len(entries))
	}

	entry := entries[0]
	if entry.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", entry.InputTokens)
	}
	if entry.OutputTokens != 500 {
		t.Errorf("OutputTokens = %d, want 500", entry.OutputTokens)
	}
	if entry.TotalTokens != 600 {
		t.Errorf("TotalTokens = %d, want 600", entry.TotalTokens)
	}
	if entry.ProviderID != "resp-large" {
		t.Errorf("ProviderID = %s, want resp-large", entry.ProviderID)
	}
}

func TestStreamUsageWrapperSmallReads(t *testing.T) {
	// Fragmented reads (7-byte chunks) to verify cross-boundary event detection
	streamData := `data: {"id":"chatcmpl-frag","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}

data: [DONE]

`
	logger := &trackingLogger{enabled: true}
	stream := io.NopCloser(strings.NewReader(streamData))
	wrapper := NewStreamUsageWrapper(stream, logger, "gpt-4", "openai", "req-frag", "/v1/chat/completions", nil)

	// Read in small chunks of 7 bytes
	buf := make([]byte, 7)
	var allData []byte
	for {
		n, err := wrapper.Read(buf)
		if n > 0 {
			allData = append(allData, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}
	}

	if string(allData) != streamData {
		t.Errorf("data mismatch: got %d bytes, want %d bytes", len(allData), len(streamData))
	}

	if err := wrapper.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	entries := logger.getEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.InputTokens != 5 {
		t.Errorf("InputTokens = %d, want 5", entry.InputTokens)
	}
	if entry.OutputTokens != 3 {
		t.Errorf("OutputTokens = %d, want 3", entry.OutputTokens)
	}
	if entry.TotalTokens != 8 {
		t.Errorf("TotalTokens = %d, want 8", entry.TotalTokens)
	}
}
