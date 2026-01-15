package auditlog

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
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
		ID:               "test-id-123",
		Timestamp:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		DurationNs:       1500000,
		Model:            "gpt-4",
		Provider:         "openai",
		StatusCode:       200,
		RequestID:        "req-123",
		ClientIP:         "192.168.1.1",
		Method:           "POST",
		Path:             "/v1/chat/completions",
		Stream:           false,
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		Data: &LogData{
			UserAgent: "test-agent",
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
	if decoded.PromptTokens != entry.PromptTokens {
		t.Errorf("PromptTokens mismatch: expected %d, got %d", entry.PromptTokens, decoded.PromptTokens)
	}
	if decoded.RequestID != entry.RequestID {
		t.Errorf("RequestID mismatch: expected %q, got %q", entry.RequestID, decoded.RequestID)
	}
}

func TestLogDataWithBodies(t *testing.T) {
	// Use interface{} types (maps) for bodies - this is how they're stored now
	requestBody := map[string]interface{}{
		"model":    "gpt-4",
		"messages": []interface{}{},
	}
	responseBody := map[string]interface{}{
		"id":      "resp-123",
		"choices": []interface{}{},
	}

	data := &LogData{
		UserAgent:    "test-agent",
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

	// Verify bodies are preserved (decoded as map[string]interface{})
	decodedReqBody, ok := decoded.RequestBody.(map[string]interface{})
	if !ok {
		t.Fatalf("RequestBody is not a map, got %T", decoded.RequestBody)
	}
	if decodedReqBody["model"] != "gpt-4" {
		t.Errorf("RequestBody model mismatch: expected gpt-4, got %v", decodedReqBody["model"])
	}

	decodedRespBody, ok := decoded.ResponseBody.(map[string]interface{})
	if !ok {
		t.Fatalf("ResponseBody is not a map, got %T", decoded.ResponseBody)
	}
	if decodedRespBody["id"] != "resp-123" {
		t.Errorf("ResponseBody id mismatch: expected resp-123, got %v", decodedRespBody["id"])
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
			ID:        fmt.Sprintf("entry-%d", i),
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

func TestIsModelInteractionPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"chat completions", "/v1/chat/completions", true},
		{"chat completions with query", "/v1/chat/completions?stream=true", true},
		{"responses", "/v1/responses", true},
		{"responses with subpath", "/v1/responses/123", true},
		{"models", "/v1/models", true},
		{"models with subpath", "/v1/models/gpt-4", true},
		{"health", "/health", false},
		{"metrics", "/metrics", false},
		{"admin", "/admin", false},
		{"root", "/", false},
		{"empty", "", false},
		{"v1 prefix only", "/v1", false},
		{"v1 other endpoint", "/v1/other", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsModelInteractionPath(tt.path)
			if result != tt.expected {
				t.Errorf("IsModelInteractionPath(%q) = %v, want %v", tt.path, result, tt.expected)
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

	// Verify usage was captured (now on entry directly, not entry.Data)
	if entry.PromptTokens != 10 {
		t.Errorf("PromptTokens: expected 10, got %d", entry.PromptTokens)
	}
	if entry.CompletionTokens != 5 {
		t.Errorf("CompletionTokens: expected 5, got %d", entry.CompletionTokens)
	}
	if entry.TotalTokens != 15 {
		t.Errorf("TotalTokens: expected 15, got %d", entry.TotalTokens)
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
		RequestID:  "req-123",
		ClientIP:   "127.0.0.1",
		Method:     "POST",
		Path:       "/v1/chat/completions",
		Stream:     false,
		Data: &LogData{
			UserAgent: "test",
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
	if !streamEntry.Stream {
		t.Error("Stream should be true")
	}
	if streamEntry.RequestID != baseEntry.RequestID {
		t.Error("RequestID not copied")
	}
	if streamEntry.ClientIP != baseEntry.ClientIP {
		t.Error("ClientIP not copied")
	}
	if streamEntry.Method != baseEntry.Method {
		t.Error("Method not copied")
	}
	if streamEntry.Path != baseEntry.Path {
		t.Error("Path not copied")
	}

	// Verify Data fields are copied
	if streamEntry.Data == nil {
		t.Fatal("Data is nil")
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

// Helper compression functions for tests
func compressGzip(data []byte) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write(data)
	_ = w.Close()
	return buf.Bytes()
}

func compressDeflate(data []byte) []byte {
	var buf bytes.Buffer
	w, _ := flate.NewWriter(&buf, flate.DefaultCompression)
	_, _ = w.Write(data)
	_ = w.Close()
	return buf.Bytes()
}

func compressBrotli(data []byte) []byte {
	var buf bytes.Buffer
	w := brotli.NewWriter(&buf)
	_, _ = w.Write(data)
	_ = w.Close()
	return buf.Bytes()
}

func TestDecompressBody(t *testing.T) {
	originalData := []byte(`{"message": "hello world", "count": 42}`)

	tests := []struct {
		name             string
		encoding         string
		compressFunc     func([]byte) []byte
		shouldDecompress bool
	}{
		{
			name:             "no encoding",
			encoding:         "",
			compressFunc:     func(b []byte) []byte { return b },
			shouldDecompress: false,
		},
		{
			name:             "identity encoding",
			encoding:         "identity",
			compressFunc:     func(b []byte) []byte { return b },
			shouldDecompress: false,
		},
		{
			name:             "gzip encoding",
			encoding:         "gzip",
			compressFunc:     compressGzip,
			shouldDecompress: true,
		},
		{
			name:             "deflate encoding",
			encoding:         "deflate",
			compressFunc:     compressDeflate,
			shouldDecompress: true,
		},
		{
			name:             "brotli encoding",
			encoding:         "br",
			compressFunc:     compressBrotli,
			shouldDecompress: true,
		},
		{
			name:             "gzip with extra spaces",
			encoding:         "  gzip  ",
			compressFunc:     compressGzip,
			shouldDecompress: true,
		},
		{
			name:             "multiple encodings (first only)",
			encoding:         "gzip, deflate",
			compressFunc:     compressGzip,
			shouldDecompress: true,
		},
		{
			name:             "unknown encoding",
			encoding:         "unknown",
			compressFunc:     func(b []byte) []byte { return b },
			shouldDecompress: false,
		},
		{
			name:             "uppercase gzip",
			encoding:         "GZIP",
			compressFunc:     compressGzip,
			shouldDecompress: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed := tt.compressFunc(originalData)
			result, decompressed := decompressBody(compressed, tt.encoding)

			if decompressed != tt.shouldDecompress {
				t.Errorf("decompressed = %v, want %v", decompressed, tt.shouldDecompress)
			}

			if tt.shouldDecompress {
				if !bytes.Equal(result, originalData) {
					t.Errorf("decompressed data mismatch: got %s, want %s", result, originalData)
				}
			}
		})
	}
}

func TestDecompressBodyInvalidData(t *testing.T) {
	// Invalid compressed data should return original
	invalidData := []byte("not valid compressed data")

	result, decompressed := decompressBody(invalidData, "gzip")
	if decompressed {
		t.Error("expected decompression to fail for invalid gzip data")
	}
	if !bytes.Equal(result, invalidData) {
		t.Error("expected original data to be returned on failure")
	}
}

func TestDecompressBodyEmptyInput(t *testing.T) {
	// Empty body should return unchanged
	result, decompressed := decompressBody([]byte{}, "gzip")
	if decompressed {
		t.Error("expected no decompression for empty body")
	}
	if len(result) != 0 {
		t.Error("expected empty result for empty input")
	}

	// Nil body should return unchanged
	result, decompressed = decompressBody(nil, "gzip")
	if decompressed {
		t.Error("expected no decompression for nil body")
	}
	if result != nil {
		t.Error("expected nil result for nil input")
	}
}
