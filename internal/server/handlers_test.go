package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"gomodel/internal/core"
	"gomodel/internal/usage"
)

// mockProvider implements core.Provider for testing
type mockProvider struct {
	err               error
	response          *core.ChatResponse
	responsesResponse *core.ResponsesResponse
	modelsResponse    *core.ModelsResponse
	streamData        string
	supportedModels   []string
}

func (m *mockProvider) Supports(model string) bool {
	for _, supported := range m.supportedModels {
		if model == supported {
			return true
		}
	}
	return false
}

func (m *mockProvider) GetProviderType(model string) string {
	if m.Supports(model) {
		return "mock"
	}
	return ""
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

func (m *mockProvider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.modelsResponse, nil
}

func (m *mockProvider) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.responsesResponse, nil
}

func (m *mockProvider) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
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
	handler := NewHandler(mock, nil, nil, nil)

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
	handler := NewHandler(mock, nil, nil, nil)

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
	handler := NewHandler(mock, nil, nil, nil)

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
	handler := NewHandler(&mockProvider{}, nil, nil, nil)

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

func TestListModels(t *testing.T) {
	mock := &mockProvider{
		modelsResponse: &core.ModelsResponse{
			Object: "list",
			Data: []core.Model{
				{
					ID:      "gpt-4o-mini",
					Object:  "model",
					Created: 1721172741,
					OwnedBy: "system",
				},
				{
					ID:      "gpt-4-turbo",
					Object:  "model",
					Created: 1712361441,
					OwnedBy: "system",
				},
			},
		},
	}

	e := echo.New()
	handler := NewHandler(mock, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListModels(c)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"object":"list"`) {
		t.Errorf("response missing object field, got: %s", body)
	}
	if !strings.Contains(body, "gpt-4o-mini") {
		t.Errorf("response missing gpt-4o-mini model, got: %s", body)
	}
	if !strings.Contains(body, "gpt-4-turbo") {
		t.Errorf("response missing gpt-4-turbo model, got: %s", body)
	}
}

func TestListModelsError(t *testing.T) {
	mock := &mockProvider{
		err: io.EOF, // Simulate an error
	}

	e := echo.New()
	handler := NewHandler(mock, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListModels(c)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "error") {
		t.Errorf("response should contain error message, got: %s", body)
	}
}

// Tests for typed error handling

func TestHandleError_ProviderError(t *testing.T) {
	mock := &mockProvider{
		supportedModels: []string{"gpt-4o-mini"},
		err:             core.NewProviderError("openai", http.StatusBadGateway, "upstream error", nil),
	}

	e := echo.New()
	handler := NewHandler(mock, nil, nil, nil)

	reqBody := `{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ChatCompletion(c)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected status 502, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "provider_error") {
		t.Errorf("response should contain error type, got: %s", body)
	}
	if !strings.Contains(body, "upstream error") {
		t.Errorf("response should contain error message, got: %s", body)
	}
}

func TestHandleError_RateLimitError(t *testing.T) {
	mock := &mockProvider{
		supportedModels: []string{"gpt-4o-mini"},
		err:             core.NewRateLimitError("openai", "rate limit exceeded"),
	}

	e := echo.New()
	handler := NewHandler(mock, nil, nil, nil)

	reqBody := `{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ChatCompletion(c)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "rate_limit_error") {
		t.Errorf("response should contain error type, got: %s", body)
	}
	if !strings.Contains(body, "rate limit exceeded") {
		t.Errorf("response should contain error message, got: %s", body)
	}
}

func TestHandleError_InvalidRequestError(t *testing.T) {
	mock := &mockProvider{
		supportedModels: []string{"gpt-4o-mini"},
		err:             core.NewInvalidRequestError("invalid parameters", nil),
	}

	e := echo.New()
	handler := NewHandler(mock, nil, nil, nil)

	reqBody := `{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hi"}]}`
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

	body := rec.Body.String()
	if !strings.Contains(body, "invalid_request_error") {
		t.Errorf("response should contain error type, got: %s", body)
	}
	if !strings.Contains(body, "invalid parameters") {
		t.Errorf("response should contain error message, got: %s", body)
	}
}

func TestHandleError_AuthenticationError(t *testing.T) {
	mock := &mockProvider{
		supportedModels: []string{"gpt-4o-mini"},
		err:             core.NewAuthenticationError("openai", "invalid API key"),
	}

	e := echo.New()
	handler := NewHandler(mock, nil, nil, nil)

	reqBody := `{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ChatCompletion(c)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "authentication_error") {
		t.Errorf("response should contain error type, got: %s", body)
	}
	if !strings.Contains(body, "invalid API key") {
		t.Errorf("response should contain error message, got: %s", body)
	}
}

func TestHandleError_NotFoundError(t *testing.T) {
	mock := &mockProvider{
		supportedModels: []string{"gpt-4o-mini"},
		err:             core.NewNotFoundError("model not found"),
	}

	e := echo.New()
	handler := NewHandler(mock, nil, nil, nil)

	reqBody := `{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ChatCompletion(c)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "not_found_error") {
		t.Errorf("response should contain error type, got: %s", body)
	}
	if !strings.Contains(body, "model not found") {
		t.Errorf("response should contain error message, got: %s", body)
	}
}

func TestHandleError_StreamingError(t *testing.T) {
	mock := &mockProvider{
		supportedModels: []string{"gpt-4o-mini"},
		err:             core.NewRateLimitError("openai", "rate limit exceeded during streaming"),
	}

	e := echo.New()
	handler := NewHandler(mock, nil, nil, nil)

	reqBody := `{"model": "gpt-4o-mini", "stream": true, "messages": [{"role": "user", "content": "Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ChatCompletion(c)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "rate_limit_error") {
		t.Errorf("response should contain error type, got: %s", body)
	}
}

func TestChatCompletion_InvalidJSON(t *testing.T) {
	mock := &mockProvider{
		supportedModels: []string{"gpt-4o-mini"},
	}

	e := echo.New()
	handler := NewHandler(mock, nil, nil, nil)

	reqBody := `{invalid json}`
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

	body := rec.Body.String()
	if !strings.Contains(body, "invalid_request_error") {
		t.Errorf("response should contain error type, got: %s", body)
	}
	if !strings.Contains(body, "invalid request body") {
		t.Errorf("response should contain error message, got: %s", body)
	}
}

func TestListModels_TypedError(t *testing.T) {
	mock := &mockProvider{
		err: core.NewProviderError("openai", http.StatusBadGateway, "failed to list models", nil),
	}

	e := echo.New()
	handler := NewHandler(mock, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListModels(c)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected status 502, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "provider_error") {
		t.Errorf("response should contain error type, got: %s", body)
	}
	if !strings.Contains(body, "failed to list models") {
		t.Errorf("response should contain error message, got: %s", body)
	}
}

// mockUsageLogger implements usage.LoggerInterface for testing.
type mockUsageLogger struct {
	config usage.Config
}

func (m *mockUsageLogger) Write(_ *usage.UsageEntry) {}
func (m *mockUsageLogger) Config() usage.Config       { return m.config }
func (m *mockUsageLogger) Close() error               { return nil }

// capturingProvider is a mockProvider that captures the request passed to StreamResponses/StreamChatCompletion.
type capturingProvider struct {
	mockProvider
	capturedChatReq      *core.ChatRequest
	capturedResponsesReq *core.ResponsesRequest
}

func (c *capturingProvider) StreamChatCompletion(_ context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	c.capturedChatReq = req
	return io.NopCloser(strings.NewReader(c.streamData)), nil
}

func (c *capturingProvider) StreamResponses(_ context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	c.capturedResponsesReq = req
	return io.NopCloser(strings.NewReader(c.streamData)), nil
}

func TestStreamingResponses_DoesNotInjectStreamOptions(t *testing.T) {
	streamData := "data: {\"type\":\"response.done\"}\n\ndata: [DONE]\n\n"
	provider := &capturingProvider{
		mockProvider: mockProvider{
			supportedModels: []string{"gpt-4o-mini"},
			streamData:      streamData,
		},
	}

	usageLog := &mockUsageLogger{
		config: usage.Config{
			Enabled:                   true,
			EnforceReturningUsageData: true,
		},
	}

	e := echo.New()
	handler := NewHandler(provider, nil, usageLog)

	// Streaming Responses request should NOT have StreamOptions injected
	reqBody := `{"model":"gpt-4o-mini","input":"Hello","stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Responses(c)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	if provider.capturedResponsesReq == nil {
		t.Fatalf("expected capturedResponsesReq to be set, got nil")
	}
	if provider.capturedResponsesReq.StreamOptions != nil {
		t.Errorf("Responses streaming should NOT have StreamOptions injected, got: %+v", provider.capturedResponsesReq.StreamOptions)
	}
}

func TestStreamingChatCompletion_InjectsStreamOptions(t *testing.T) {
	streamData := "data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n"
	provider := &capturingProvider{
		mockProvider: mockProvider{
			supportedModels: []string{"gpt-4o-mini"},
			streamData:      streamData,
		},
	}

	usageLog := &mockUsageLogger{
		config: usage.Config{
			Enabled:                   true,
			EnforceReturningUsageData: true,
		},
	}

	e := echo.New()
	handler := NewHandler(provider, nil, usageLog)

	// Streaming ChatCompletion request SHOULD have StreamOptions injected
	reqBody := `{"model":"gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"Hi"}]}`
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

	if provider.capturedChatReq.StreamOptions == nil {
		t.Fatal("ChatCompletion streaming should have StreamOptions injected")
	}

	if !provider.capturedChatReq.StreamOptions.IncludeUsage {
		t.Error("ChatCompletion streaming should have IncludeUsage=true")
	}
}
