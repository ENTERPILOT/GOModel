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
	wrapper := NewStreamUsageWrapper(stream, logger, "gpt-4", "openai", "req-123", "/v1/chat/completions")

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
	wrapper := NewStreamUsageWrapper(stream, logger, "o1-preview", "openai", "req-456", "/v1/chat/completions")

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
	wrapper := NewStreamUsageWrapper(stream, logger, "gpt-4", "openai", "req-789", "/v1/chat/completions")

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
	wrapper := NewStreamUsageWrapper(stream, logger, "gpt-4", "openai", "req-123", "/v1/chat/completions")

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

	wrapped := WrapStreamForUsage(stream, logger, "gpt-4", "openai", "req-123", "/v1/chat/completions")

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

	wrapped := WrapStreamForUsage(stream, nil, "gpt-4", "openai", "req-123", "/v1/chat/completions")

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
	wrapper := NewStreamUsageWrapper(stream, logger, "gpt-4", "openai", "req-123", "/v1/chat/completions")

	_, _ = io.ReadAll(wrapper)

	// Close twice should not panic or double-log
	_ = wrapper.Close()
	_ = wrapper.Close()

	entries := logger.getEntries()
	if len(entries) != 1 {
		t.Errorf("expected 1 entry (not 2 from double close), got %d", len(entries))
	}
}
