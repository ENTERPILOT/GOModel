package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gomodel/internal/core"
)

func TestNew(t *testing.T) {
	apiKey := "test-api-key"
	provider := New(apiKey)

	if provider.apiKey != apiKey {
		t.Errorf("apiKey = %q, want %q", provider.apiKey, apiKey)
	}
	if provider.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", provider.baseURL, defaultBaseURL)
	}
	if provider.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestSupports(t *testing.T) {
	provider := New("test-api-key")

	tests := []struct {
		model    string
		expected bool
	}{
		{"claude-3-5-sonnet-20241022", true},
		{"claude-3-opus-20240229", true},
		{"claude-3-haiku-20240307", true},
		{"gpt-4o", false},
		{"gpt-4o-mini", false},
		{"o1-preview", false},
		{"random-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := provider.Supports(tt.model)
			if result != tt.expected {
				t.Errorf("Supports(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestChatCompletion(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectedError bool
		checkResponse func(*testing.T, *core.ChatResponse)
	}{
		{
			name:       "successful request",
			statusCode: http.StatusOK,
			responseBody: `{
				"id": "msg_123",
				"type": "message",
				"role": "assistant",
				"model": "claude-3-5-sonnet-20241022",
				"content": [{
					"type": "text",
					"text": "Hello! How can I help you today?"
				}],
				"stop_reason": "end_turn",
				"usage": {
					"input_tokens": 10,
					"output_tokens": 20
				}
			}`,
			expectedError: false,
			checkResponse: func(t *testing.T, resp *core.ChatResponse) {
				if resp.ID != "msg_123" {
					t.Errorf("ID = %q, want %q", resp.ID, "msg_123")
				}
				if resp.Model != "claude-3-5-sonnet-20241022" {
					t.Errorf("Model = %q, want %q", resp.Model, "claude-3-5-sonnet-20241022")
				}
				if len(resp.Choices) != 1 {
					t.Fatalf("len(Choices) = %d, want 1", len(resp.Choices))
				}
				if resp.Choices[0].Message.Content != "Hello! How can I help you today?" {
					t.Errorf("Message content = %q, want %q", resp.Choices[0].Message.Content, "Hello! How can I help you today?")
				}
				if resp.Usage.PromptTokens != 10 {
					t.Errorf("PromptTokens = %d, want 10", resp.Usage.PromptTokens)
				}
				if resp.Usage.CompletionTokens != 20 {
					t.Errorf("CompletionTokens = %d, want 20", resp.Usage.CompletionTokens)
				}
				if resp.Usage.TotalTokens != 30 {
					t.Errorf("TotalTokens = %d, want 30", resp.Usage.TotalTokens)
				}
			},
		},
		{
			name:          "API error - unauthorized",
			statusCode:    http.StatusUnauthorized,
			responseBody:  `{"type": "error", "error": {"type": "authentication_error", "message": "Invalid API key"}}`,
			expectedError: true,
		},
		{
			name:          "rate limit error",
			statusCode:    http.StatusTooManyRequests,
			responseBody:  `{"type": "error", "error": {"type": "rate_limit_error", "message": "Rate limit exceeded"}}`,
			expectedError: true,
		},
		{
			name:          "server error",
			statusCode:    http.StatusInternalServerError,
			responseBody:  `{"type": "error", "error": {"type": "api_error", "message": "Internal server error"}}`,
			expectedError: true,
		},
		{
			name:          "bad request error",
			statusCode:    http.StatusBadRequest,
			responseBody:  `{"type": "error", "error": {"type": "invalid_request_error", "message": "Invalid request"}}`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request headers
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("Content-Type = %q, want %q", r.Header.Get("Content-Type"), "application/json")
				}
				apiKey := r.Header.Get("x-api-key")
				if apiKey == "" {
					t.Error("x-api-key header should not be empty")
				}
				if r.Header.Get("anthropic-version") != anthropicAPIVersion {
					t.Errorf("anthropic-version = %q, want %q", r.Header.Get("anthropic-version"), anthropicAPIVersion)
				}

				// Verify request path
				if r.URL.Path != "/messages" {
					t.Errorf("Path = %q, want %q", r.URL.Path, "/messages")
				}

				// Verify request body
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("failed to read request body: %v", err)
				}
				var req anthropicRequest
				if err := json.Unmarshal(body, &req); err != nil {
					t.Fatalf("failed to unmarshal request: %v", err)
				}

				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			provider := New("test-api-key")
			provider.baseURL = server.URL

			req := &core.ChatRequest{
				Model: "claude-3-5-sonnet-20241022",
				Messages: []core.Message{
					{Role: "user", Content: "Hello"},
				},
			}

			resp, err := provider.ChatCompletion(context.Background(), req)

			if tt.expectedError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if tt.checkResponse != nil {
					tt.checkResponse(t, resp)
				}
			}
		})
	}
}

func TestStreamChatCompletion(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectedError bool
		checkStream   func(*testing.T, io.ReadCloser)
	}{
		{
			name:       "successful streaming request",
			statusCode: http.StatusOK,
			responseBody: `event: message_start
data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","model":"claude-3-5-sonnet-20241022","content":[],"stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"!"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}

event: message_stop
data: {"type":"message_stop"}
`,
			expectedError: false,
			checkStream: func(t *testing.T, body io.ReadCloser) {
				if body == nil {
					t.Fatal("body should not be nil")
				}
				defer body.Close()

				// Read and verify the streaming response
				respBody, err := io.ReadAll(body)
				if err != nil {
					t.Fatalf("failed to read response body: %v", err)
				}

				// The response should be converted to OpenAI format
				responseStr := string(respBody)
				if !strings.Contains(responseStr, "data:") {
					t.Error("response should contain SSE data")
				}
				if !strings.Contains(responseStr, "[DONE]") {
					t.Error("response should end with [DONE]")
				}
			},
		},
		{
			name:          "API error - unauthorized",
			statusCode:    http.StatusUnauthorized,
			responseBody:  `{"type": "error", "error": {"type": "authentication_error", "message": "Invalid API key"}}`,
			expectedError: true,
		},
		{
			name:          "rate limit error",
			statusCode:    http.StatusTooManyRequests,
			responseBody:  `{"type": "error", "error": {"type": "rate_limit_error", "message": "Rate limit exceeded"}}`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request headers
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("Content-Type = %q, want %q", r.Header.Get("Content-Type"), "application/json")
				}
				apiKey := r.Header.Get("x-api-key")
				if apiKey == "" {
					t.Error("x-api-key header should not be empty")
				}

				// Verify stream is set in request body
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("failed to read request body: %v", err)
				}
				var req anthropicRequest
				if err := json.Unmarshal(body, &req); err != nil {
					t.Fatalf("failed to unmarshal request: %v", err)
				}
				if !req.Stream {
					t.Error("Stream should be true in request")
				}

				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			provider := New("test-api-key")
			provider.baseURL = server.URL

			req := &core.ChatRequest{
				Model: "claude-3-5-sonnet-20241022",
				Messages: []core.Message{
					{Role: "user", Content: "Hello"},
				},
			}

			body, err := provider.StreamChatCompletion(context.Background(), req)

			if tt.expectedError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if tt.checkStream != nil {
					tt.checkStream(t, body)
				}
			}
		})
	}
}

func TestListModels(t *testing.T) {
	provider := New("test-api-key")

	resp, err := provider.ListModels(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Object != "list" {
		t.Errorf("Object = %q, want %q", resp.Object, "list")
	}

	if len(resp.Data) == 0 {
		t.Error("Data should not be empty")
	}

	// Verify that all models have the correct fields
	for _, model := range resp.Data {
		if model.ID == "" {
			t.Error("Model ID should not be empty")
		}
		if !strings.HasPrefix(model.ID, "claude-") {
			t.Errorf("Model ID %q should start with 'claude-'", model.ID)
		}
		if model.Object != "model" {
			t.Errorf("Model.Object = %q, want %q", model.Object, "model")
		}
		if model.OwnedBy != "anthropic" {
			t.Errorf("Model.OwnedBy = %q, want %q", model.OwnedBy, "anthropic")
		}
		if model.Created == 0 {
			t.Error("Model.Created should not be zero")
		}
	}

	// Verify some expected models are present
	expectedModels := map[string]bool{
		"claude-3-5-sonnet-20241022": false,
		"claude-3-opus-20240229":     false,
		"claude-3-haiku-20240307":    false,
	}

	for _, model := range resp.Data {
		if _, ok := expectedModels[model.ID]; ok {
			expectedModels[model.ID] = true
		}
	}

	for model, found := range expectedModels {
		if !found {
			t.Errorf("Expected model %q not found in response", model)
		}
	}
}

func TestChatCompletionWithContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow response
		<-r.Context().Done()
		w.WriteHeader(http.StatusRequestTimeout)
	}))
	defer server.Close()

	provider := New("test-api-key")
	provider.baseURL = server.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := &core.ChatRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []core.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	_, err := provider.ChatCompletion(ctx, req)
	if err == nil {
		t.Error("expected error when context is cancelled, got nil")
	}
}

func TestConvertToAnthropicRequest(t *testing.T) {
	temp := 0.7
	maxTokens := 1024

	tests := []struct {
		name    string
		input   *core.ChatRequest
		checkFn func(*testing.T, *anthropicRequest)
	}{
		{
			name: "basic request",
			input: &core.ChatRequest{
				Model: "claude-3-5-sonnet-20241022",
				Messages: []core.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			checkFn: func(t *testing.T, req *anthropicRequest) {
				if req.Model != "claude-3-5-sonnet-20241022" {
					t.Errorf("Model = %q, want %q", req.Model, "claude-3-5-sonnet-20241022")
				}
				if len(req.Messages) != 1 {
					t.Errorf("len(Messages) = %d, want 1", len(req.Messages))
				}
				if req.Messages[0].Content != "Hello" {
					t.Errorf("Message content = %q, want %q", req.Messages[0].Content, "Hello")
				}
				if req.MaxTokens != 4096 {
					t.Errorf("MaxTokens = %d, want 4096", req.MaxTokens)
				}
			},
		},
		{
			name: "request with system message",
			input: &core.ChatRequest{
				Model: "claude-3-opus-20240229",
				Messages: []core.Message{
					{Role: "system", Content: "You are a helpful assistant"},
					{Role: "user", Content: "Hello"},
				},
			},
			checkFn: func(t *testing.T, req *anthropicRequest) {
				if req.System != "You are a helpful assistant" {
					t.Errorf("System = %q, want %q", req.System, "You are a helpful assistant")
				}
				if len(req.Messages) != 1 {
					t.Errorf("len(Messages) = %d, want 1 (system should be extracted)", len(req.Messages))
				}
			},
		},
		{
			name: "request with parameters",
			input: &core.ChatRequest{
				Model:       "claude-3-5-sonnet-20241022",
				Temperature: &temp,
				MaxTokens:   &maxTokens,
				Messages: []core.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			checkFn: func(t *testing.T, req *anthropicRequest) {
				if req.Temperature == nil || *req.Temperature != 0.7 {
					t.Errorf("Temperature = %v, want 0.7", req.Temperature)
				}
				if req.MaxTokens != 1024 {
					t.Errorf("MaxTokens = %d, want 1024", req.MaxTokens)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToAnthropicRequest(tt.input)
			tt.checkFn(t, result)
		})
	}
}

func TestConvertFromAnthropicResponse(t *testing.T) {
	resp := &anthropicResponse{
		ID:    "msg_123",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-3-5-sonnet-20241022",
		Content: []anthropicContent{
			{Type: "text", Text: "Hello! How can I help you today?"},
		},
		StopReason: "end_turn",
		Usage: anthropicUsage{
			InputTokens:  10,
			OutputTokens: 20,
		},
	}

	result := convertFromAnthropicResponse(resp)

	if result.ID != "msg_123" {
		t.Errorf("ID = %q, want %q", result.ID, "msg_123")
	}
	if result.Object != "chat.completion" {
		t.Errorf("Object = %q, want %q", result.Object, "chat.completion")
	}
	if result.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("Model = %q, want %q", result.Model, "claude-3-5-sonnet-20241022")
	}
	if len(result.Choices) != 1 {
		t.Fatalf("len(Choices) = %d, want 1", len(result.Choices))
	}
	if result.Choices[0].Message.Content != "Hello! How can I help you today?" {
		t.Errorf("Message content = %q, want %q", result.Choices[0].Message.Content, "Hello! How can I help you today?")
	}
	if result.Choices[0].Message.Role != "assistant" {
		t.Errorf("Message role = %q, want %q", result.Choices[0].Message.Role, "assistant")
	}
	if result.Choices[0].FinishReason != "end_turn" {
		t.Errorf("FinishReason = %q, want %q", result.Choices[0].FinishReason, "end_turn")
	}
	if result.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", result.Usage.PromptTokens)
	}
	if result.Usage.CompletionTokens != 20 {
		t.Errorf("CompletionTokens = %d, want 20", result.Usage.CompletionTokens)
	}
	if result.Usage.TotalTokens != 30 {
		t.Errorf("TotalTokens = %d, want 30", result.Usage.TotalTokens)
	}
}
