package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"heavymodel/internal/core"
)

// mockProvider implements core.Provider for testing
type mockProvider struct {
	supportedModels []string
	response        *core.ChatResponse
	streamData      string
	err             error
}

func (m *mockProvider) Supports(model string) bool {
	for _, supported := range m.supportedModels {
		if model == supported {
			return true
		}
	}
	return false
}

func (m *mockProvider) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockProvider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	return io.NopCloser(strings.NewReader(m.streamData)), nil
}

func TestChatCompletion(t *testing.T) {
	mock := &mockProvider{
		supportedModels: []string{"gpt-4o-mini"},
		response: &core.ChatResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "gpt-4o-mini",
			Choices: []core.Choice{
				{
					Index:        0,
					Message:      core.Message{Role: "assistant", Content: "Hello!"},
					FinishReason: "stop",
				},
			},
			Usage: core.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		},
	}

	e := echo.New()
	handler := NewHandler(mock)

	reqBody := `{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ChatCompletion(c)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "chatcmpl-123") {
		t.Errorf("response missing expected ID, got: %s", body)
	}
	if !strings.Contains(body, "Hello!") {
		t.Errorf("response missing expected content, got: %s", body)
	}
}

func TestChatCompletionUnsupportedModel(t *testing.T) {
	mock := &mockProvider{
		supportedModels: []string{"gpt-4o-mini"},
	}

	e := echo.New()
	handler := NewHandler(mock)

	reqBody := `{"model": "unsupported-model", "messages": [{"role": "user", "content": "Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ChatCompletion(c)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestChatCompletionStreaming(t *testing.T) {
	streamData := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}

data: [DONE]

`
	mock := &mockProvider{
		supportedModels: []string{"gpt-4o-mini"},
		streamData:      streamData,
	}

	e := echo.New()
	handler := NewHandler(mock)

	reqBody := `{"model": "gpt-4o-mini", "stream": true, "messages": [{"role": "user", "content": "Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ChatCompletion(c)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", contentType)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "data:") {
		t.Errorf("response should contain SSE data, got: %s", body)
	}
	if !strings.Contains(body, "[DONE]") {
		t.Errorf("response should contain [DONE], got: %s", body)
	}
}

func TestHealth(t *testing.T) {
	e := echo.New()
	handler := NewHandler(&mockProvider{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Health(c)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "ok") {
		t.Errorf("expected ok status in body")
	}
}
