package auditlog

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRedactHeaders(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name:     "nil headers",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty headers",
			input:    map[string]string{},
			expected: map[string]string{},
		},
		{
			name: "no sensitive headers",
			input: map[string]string{
				"Content-Type": "application/json",
				"Accept":       "application/json",
			},
			expected: map[string]string{
				"Content-Type": "application/json",
				"Accept":       "application/json",
			},
		},
		{
			name: "redact authorization",
			input: map[string]string{
				"Authorization": "Bearer sk-secret-key",
				"Content-Type":  "application/json",
			},
			expected: map[string]string{
				"Authorization": "[REDACTED]",
				"Content-Type":  "application/json",
			},
		},
		{
			name: "redact multiple sensitive headers",
			input: map[string]string{
				"Authorization":       "Bearer token",
				"X-Api-Key":           "secret-key",
				"Cookie":              "session=abc123",
				"Content-Type":        "application/json",
				"X-Auth-Token":        "some-token",
				"Proxy-Authorization": "Basic creds",
			},
			expected: map[string]string{
				"Authorization":       "[REDACTED]",
				"X-Api-Key":           "[REDACTED]",
				"Cookie":              "[REDACTED]",
				"Content-Type":        "application/json",
				"X-Auth-Token":        "[REDACTED]",
				"Proxy-Authorization": "[REDACTED]",
			},
		},
		{
			name: "case insensitive redaction",
			input: map[string]string{
				"AUTHORIZATION": "Bearer token",
				"x-api-key":     "secret",
				"X-API-KEY":     "another-secret",
			},
			expected: map[string]string{
				"AUTHORIZATION": "[REDACTED]",
				"x-api-key":     "[REDACTED]",
				"X-API-KEY":     "[REDACTED]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RedactHeaders(tt.input)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d headers, got %d", len(tt.expected), len(result))
			}

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("header %q: expected %q, got %q", k, v, result[k])
				}
			}
		})
	}
}

func TestLogEntryJSON(t *testing.T) {
	entry := &LogEntry{
		ID:         "test-id-123",
		Timestamp:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		DurationNs: 1500000,
		Model:      "gpt-4",
		Provider:   "openai",
		StatusCode: 200,
		Data: &LogData{
			RequestID:        "req-123",
			ClientIP:         "192.168.1.1",
			UserAgent:        "test-agent",
			Method:           "POST",
			Path:             "/v1/chat/completions",
			Stream:           false,
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal entry: %v", err)
	}

	// Test JSON unmarshaling
	var decoded LogEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal entry: %v", err)
	}

	// Verify fields
	if decoded.ID != entry.ID {
		t.Errorf("ID mismatch: expected %q, got %q", entry.ID, decoded.ID)
	}
	if decoded.Model != entry.Model {
		t.Errorf("Model mismatch: expected %q, got %q", entry.Model, decoded.Model)
	}
	if decoded.Provider != entry.Provider {
		t.Errorf("Provider mismatch: expected %q, got %q", entry.Provider, decoded.Provider)
	}
	if decoded.StatusCode != entry.StatusCode {
		t.Errorf("StatusCode mismatch: expected %d, got %d", entry.StatusCode, decoded.StatusCode)
	}
	if decoded.Data == nil {
		t.Fatal("Data is nil after unmarshal")
	}
	if decoded.Data.PromptTokens != entry.Data.PromptTokens {
		t.Errorf("PromptTokens mismatch: expected %d, got %d", entry.Data.PromptTokens, decoded.Data.PromptTokens)
	}
}

func TestLogDataWithBodies(t *testing.T) {
	requestBody := json.RawMessage(`{"model":"gpt-4","messages":[]}`)
	responseBody := json.RawMessage(`{"id":"resp-123","choices":[]}`)

	data := &LogData{
		RequestID:    "req-123",
		Method:       "POST",
		Path:         "/v1/chat/completions",
		RequestBody:  requestBody,
		ResponseBody: responseBody,
	}

	// Marshal and unmarshal
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded LogData
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify bodies are preserved
	if string(decoded.RequestBody) != string(requestBody) {
		t.Errorf("RequestBody mismatch: expected %s, got %s", requestBody, decoded.RequestBody)
	}
	if string(decoded.ResponseBody) != string(responseBody) {
		t.Errorf("ResponseBody mismatch: expected %s, got %s", responseBody, decoded.ResponseBody)
	}
}

// mockStore implements LogStore for testing
type mockStore struct {
	mu      sync.Mutex
	entries []*LogEntry
	closed  bool
}

func (m *mockStore) WriteBatch(_ context.Context, entries []*LogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entries...)
	return nil
}

func (m *mockStore) Flush(_ context.Context) error {
	return nil
}

func (m *mockStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockStore) getEntries() []*LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.entries
}

func (m *mockStore) isClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func TestLogger(t *testing.T) {
	store := &mockStore{}
	cfg := Config{
		Enabled:       true,
		BufferSize:    10,
		FlushInterval: 100 * time.Millisecond,
	}

	logger := NewLogger(store, cfg)
	defer logger.Close()

	// Write some entries
	for i := 0; i < 5; i++ {
		logger.Write(&LogEntry{
			ID:        "entry-" + string(rune('0'+i)),
			Timestamp: time.Now(),
			Model:     "test-model",
		})
	}

	// Wait for flush
	time.Sleep(200 * time.Millisecond)

	// Verify entries were written
	if len(store.getEntries()) != 5 {
		t.Errorf("expected 5 entries, got %d", len(store.getEntries()))
	}
}

func TestLoggerClose(t *testing.T) {
	store := &mockStore{}
	cfg := Config{
		Enabled:       true,
		BufferSize:    100,
		FlushInterval: 10 * time.Second, // Long interval to test close flushes
	}

	logger := NewLogger(store, cfg)

	// Write entry
	logger.Write(&LogEntry{
		ID:        "test-entry",
		Timestamp: time.Now(),
	})

	// Close should flush
	logger.Close()

	// Verify entry was flushed
	if len(store.getEntries()) != 1 {
		t.Errorf("expected 1 entry after close, got %d", len(store.getEntries()))
	}

	// Verify store was closed
	if !store.isClosed() {
		t.Error("store was not closed")
	}
}

func TestNoopLogger(t *testing.T) {
	logger := &NoopLogger{}

	// Should not panic
	logger.Write(&LogEntry{ID: "test"})
	logger.Close()

	cfg := logger.Config()
	if cfg.Enabled {
		t.Error("noop logger should report as disabled")
	}
}

func TestSkipLoggingPaths(t *testing.T) {
	tests := []struct {
		path   string
		skip   bool
	}{
		{"/health", true},
		{"/health/", true},
		{"/metrics", true},
		{"/metrics/prometheus", true},
		{"/favicon.ico", true},
		{"/v1/chat/completions", false},
		{"/v1/models", false},
		{"/v1/responses", false},
		{"/", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := SkipLoggingPaths(tt.path)
			if result != tt.skip {
				t.Errorf("SkipLoggingPaths(%q) = %v, want %v", tt.path, result, tt.skip)
			}
		})
	}
}

func TestParseUsageFromSSE(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Usage
	}{
		{
			name:     "empty input",
			input:    "",
			expected: nil,
		},
		{
			name:     "no usage data",
			input:    "data: {\"id\":\"chatcmpl-123\"}\n\ndata: [DONE]\n\n",
			expected: nil,
		},
		{
			name: "with usage data",
			input: `data: {"id":"chatcmpl-123","choices":[]}

data: {"id":"chatcmpl-123","usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}

data: [DONE]

`,
			expected: &Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		},
		{
			name: "usage only in last non-DONE event",
			input: `data: {"choices":[{"delta":{"content":"Hello"}}]}

data: {"choices":[{"delta":{"content":" world"}}]}

data: {"choices":[],"usage":{"prompt_tokens":5,"completion_tokens":10,"total_tokens":15}}

data: [DONE]

`,
			expected: &Usage{
				PromptTokens:     5,
				CompletionTokens: 10,
				TotalTokens:      15,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseUsageFromSSE([]byte(tt.input))

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatalf("expected usage data, got nil")
			}

			if result.PromptTokens != tt.expected.PromptTokens {
				t.Errorf("PromptTokens: expected %d, got %d", tt.expected.PromptTokens, result.PromptTokens)
			}
			if result.CompletionTokens != tt.expected.CompletionTokens {
				t.Errorf("CompletionTokens: expected %d, got %d", tt.expected.CompletionTokens, result.CompletionTokens)
			}
			if result.TotalTokens != tt.expected.TotalTokens {
				t.Errorf("TotalTokens: expected %d, got %d", tt.expected.TotalTokens, result.TotalTokens)
			}
		})
	}
}

func TestStreamLogWrapper(t *testing.T) {
	// Create a mock stream with usage data
	streamContent := `data: {"id":"chatcmpl-123","choices":[{"delta":{"content":"Hello"}}]}

data: {"id":"chatcmpl-123","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]

`
	stream := io.NopCloser(strings.NewReader(streamContent))

	// Create mock logger and entry
	store := &mockStore{}
	cfg := Config{
		Enabled:       true,
		BufferSize:    10,
		FlushInterval: 100 * time.Millisecond,
	}
	logger := NewLogger(store, cfg)
	defer logger.Close()

	entry := &LogEntry{
		ID:        "test-entry",
		Timestamp: time.Now(),
		Model:     "gpt-4",
		Data:      &LogData{},
	}

	// Wrap the stream
	wrapper := NewStreamLogWrapper(stream, logger, entry)

	// Read all content
	var buf bytes.Buffer
	_, err := io.Copy(&buf, wrapper)
	if err != nil {
		t.Fatalf("failed to read stream: %v", err)
	}

	// Close wrapper to trigger usage parsing and logging
	if err := wrapper.Close(); err != nil {
		t.Fatalf("failed to close wrapper: %v", err)
	}

	// Verify usage was captured
	if entry.Data.PromptTokens != 10 {
		t.Errorf("PromptTokens: expected 10, got %d", entry.Data.PromptTokens)
	}
	if entry.Data.CompletionTokens != 5 {
		t.Errorf("CompletionTokens: expected 5, got %d", entry.Data.CompletionTokens)
	}
	if entry.Data.TotalTokens != 15 {
		t.Errorf("TotalTokens: expected 15, got %d", entry.Data.TotalTokens)
	}

	// Wait for async write
	time.Sleep(200 * time.Millisecond)

	// Verify entry was logged
	if len(store.getEntries()) != 1 {
		t.Errorf("expected 1 entry, got %d", len(store.getEntries()))
	}
}

func TestWrapStreamForLogging(t *testing.T) {
	stream := io.NopCloser(strings.NewReader("test"))

	// Test with nil logger
	result := WrapStreamForLogging(stream, nil, nil)
	if result != stream {
		t.Error("expected original stream with nil logger")
	}

	// Test with disabled logger
	noopLogger := &NoopLogger{}
	result = WrapStreamForLogging(stream, noopLogger, &LogEntry{})
	if result != stream {
		t.Error("expected original stream with disabled logger")
	}
}

func TestCreateStreamEntry(t *testing.T) {
	// Test nil input
	result := CreateStreamEntry(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}

	// Test with valid entry
	baseEntry := &LogEntry{
		ID:         "test-id",
		Timestamp:  time.Now(),
		DurationNs: 1000,
		Model:      "gpt-4",
		Provider:   "openai",
		StatusCode: 200,
		Data: &LogData{
			RequestID:  "req-123",
			ClientIP:   "127.0.0.1",
			UserAgent:  "test",
			Method:     "POST",
			Path:       "/v1/chat/completions",
			Stream:     false,
			RequestHeaders: map[string]string{
				"Content-Type": "application/json",
			},
		},
	}

	streamEntry := CreateStreamEntry(baseEntry)
	if streamEntry == nil {
		t.Fatal("expected non-nil stream entry")
	}

	// Verify fields are copied
	if streamEntry.ID != baseEntry.ID {
		t.Errorf("ID mismatch")
	}
	if streamEntry.Model != baseEntry.Model {
		t.Errorf("Model mismatch")
	}
	if streamEntry.Data == nil {
		t.Fatal("Data is nil")
	}
	if !streamEntry.Data.Stream {
		t.Error("Stream should be true")
	}
	if streamEntry.Data.RequestID != baseEntry.Data.RequestID {
		t.Error("RequestID not copied")
	}

	// Verify headers are copied (not same reference)
	if streamEntry.Data.RequestHeaders == nil {
		t.Fatal("RequestHeaders is nil")
	}
	baseEntry.Data.RequestHeaders["New"] = "value"
	if streamEntry.Data.RequestHeaders["New"] == "value" {
		t.Error("Headers should be a copy, not same reference")
	}
}

func TestHashAPIKey(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		wantEmpty  bool
	}{
		{
			name:       "empty header",
			authHeader: "",
			wantEmpty:  true,
		},
		{
			name:       "Bearer only",
			authHeader: "Bearer ",
			wantEmpty:  true,
		},
		{
			name:       "valid Bearer token",
			authHeader: "Bearer sk-test-key-123",
			wantEmpty:  false,
		},
		{
			name:       "token without Bearer prefix",
			authHeader: "sk-test-key-123",
			wantEmpty:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hashAPIKey(tt.authHeader)
			if tt.wantEmpty {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
			} else {
				if result == "" {
					t.Error("expected non-empty hash")
				}
				if len(result) != 8 {
					t.Errorf("expected 8 character hash, got %d characters", len(result))
				}
			}
		})
	}

	// Test consistency - same input should produce same hash
	hash1 := hashAPIKey("Bearer test-key")
	hash2 := hashAPIKey("Bearer test-key")
	if hash1 != hash2 {
		t.Error("same input should produce same hash")
	}

	// Test different inputs produce different hashes
	hash3 := hashAPIKey("Bearer different-key")
	if hash1 == hash3 {
		t.Error("different inputs should produce different hashes")
	}
}
