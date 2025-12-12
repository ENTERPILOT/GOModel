package openai

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
		{"gpt-4", true},
		{"gpt-4-turbo", true},
		{"gpt-4o", true},
		{"gpt-4o-mini", true},
		{"gpt-3.5-turbo", true},
		{"o1-preview", true},
		{"o1-mini", true},
		{"claude-3-5-sonnet-20241022", false},
		{"claude-3-opus-20240229", false},
		{"random-model", false},
		{"mistral-large", false},
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
				"id": "chatcmpl-123",
				"object": "chat.completion",
				"created": 1677652288,
				"model": "gpt-4o",
				"choices": [{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "Hello! How can I help you today?"
					},
					"finish_reason": "stop"
				}],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 20,
					"total_tokens": 30
				}
			}`,
			expectedError: false,
			checkResponse: func(t *testing.T, resp *core.ChatResponse) {
				if resp.ID != "chatcmpl-123" {
					t.Errorf("ID = %q, want %q", resp.ID, "chatcmpl-123")
				}
				if resp.Model != "gpt-4o" {
					t.Errorf("Model = %q, want %q", resp.Model, "gpt-4o")
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
			name:          "API error",
			statusCode:    http.StatusUnauthorized,
			responseBody:  `{"error": {"message": "Invalid API key"}}`,
			expectedError: true,
		},
		{
			name:          "rate limit error",
			statusCode:    http.StatusTooManyRequests,
			responseBody:  `{"error": {"message": "Rate limit exceeded"}}`,
			expectedError: true,
		},
		{
			name:          "server error",
			statusCode:    http.StatusInternalServerError,
			responseBody:  `{"error": {"message": "Internal server error"}}`,
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
				authHeader := r.Header.Get("Authorization")
				if !strings.HasPrefix(authHeader, "Bearer ") {
					t.Errorf("Authorization header should start with 'Bearer '")
				}

				// Verify request body
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("failed to read request body: %v", err)
				}
				var req core.ChatRequest
				if err := json.Unmarshal(body, &req); err != nil {
					t.Fatalf("failed to unmarshal request: %v", err)
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			provider := New("test-api-key")
			provider.baseURL = server.URL

			req := &core.ChatRequest{
				Model: "gpt-4o",
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
	}{
		{
			name:       "successful streaming request",
			statusCode: http.StatusOK,
			responseBody: `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}

data: [DONE]
`,
			expectedError: false,
		},
		{
			name:          "API error",
			statusCode:    http.StatusUnauthorized,
			responseBody:  `{"error": {"message": "Invalid API key"}}`,
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
				authHeader := r.Header.Get("Authorization")
				if !strings.HasPrefix(authHeader, "Bearer ") {
					t.Errorf("Authorization header should start with 'Bearer '")
				}

				// Verify stream is set in request body
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("failed to read request body: %v", err)
				}
				var req core.ChatRequest
				if err := json.Unmarshal(body, &req); err != nil {
					t.Fatalf("failed to unmarshal request: %v", err)
				}
				if !req.Stream {
					t.Error("Stream should be true in request")
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			provider := New("test-api-key")
			provider.baseURL = server.URL

			req := &core.ChatRequest{
				Model: "gpt-4o",
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
				if body == nil {
					t.Fatal("body should not be nil")
				}
				defer func() { _ = body.Close() }()

				// Read and verify the streaming response
				respBody, err := io.ReadAll(body)
				if err != nil {
					t.Fatalf("failed to read response body: %v", err)
				}
				if string(respBody) != tt.responseBody {
					t.Errorf("response body = %q, want %q", string(respBody), tt.responseBody)
				}
			}
		})
	}
}

func TestListModels(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectedError bool
		checkResponse func(*testing.T, *core.ModelsResponse)
	}{
		{
			name:       "successful request",
			statusCode: http.StatusOK,
			responseBody: `{
				"object": "list",
				"data": [
					{
						"id": "gpt-4o",
						"object": "model",
						"created": 1687882411,
						"owned_by": "openai"
					},
					{
						"id": "gpt-4",
						"object": "model",
						"created": 1687882410,
						"owned_by": "openai"
					}
				]
			}`,
			expectedError: false,
			checkResponse: func(t *testing.T, resp *core.ModelsResponse) {
				if resp.Object != "list" {
					t.Errorf("Object = %q, want %q", resp.Object, "list")
				}
				if len(resp.Data) != 2 {
					t.Fatalf("len(Data) = %d, want 2", len(resp.Data))
				}
				if resp.Data[0].ID != "gpt-4o" {
					t.Errorf("Data[0].ID = %q, want %q", resp.Data[0].ID, "gpt-4o")
				}
				if resp.Data[0].OwnedBy != "openai" {
					t.Errorf("Data[0].OwnedBy = %q, want %q", resp.Data[0].OwnedBy, "openai")
				}
			},
		},
		{
			name:          "API error",
			statusCode:    http.StatusUnauthorized,
			responseBody:  `{"error": {"message": "Invalid API key"}}`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and path
				if r.Method != http.MethodGet {
					t.Errorf("Method = %q, want %q", r.Method, http.MethodGet)
				}
				if r.URL.Path != "/models" {
					t.Errorf("Path = %q, want %q", r.URL.Path, "/models")
				}

				// Verify authorization header
				authHeader := r.Header.Get("Authorization")
				if !strings.HasPrefix(authHeader, "Bearer ") {
					t.Errorf("Authorization header should start with 'Bearer '")
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			provider := New("test-api-key")
			provider.baseURL = server.URL

			resp, err := provider.ListModels(context.Background())

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
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	_, err := provider.ChatCompletion(ctx, req)
	if err == nil {
		t.Error("expected error when context is cancelled, got nil")
	}
}

func TestResponses(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectedError bool
		checkResponse func(*testing.T, *core.ResponsesResponse)
	}{
		{
			name:       "successful request with string input",
			statusCode: http.StatusOK,
			responseBody: `{
				"id": "resp_123",
				"object": "response",
				"created_at": 1677652288,
				"model": "gpt-4o",
				"status": "completed",
				"output": [{
					"id": "msg_123",
					"type": "message",
					"role": "assistant",
					"status": "completed",
					"content": [{
						"type": "output_text",
						"text": "Hello! How can I help you today?"
					}]
				}],
				"usage": {
					"input_tokens": 10,
					"output_tokens": 20,
					"total_tokens": 30
				}
			}`,
			expectedError: false,
			checkResponse: func(t *testing.T, resp *core.ResponsesResponse) {
				if resp.ID != "resp_123" {
					t.Errorf("ID = %q, want %q", resp.ID, "resp_123")
				}
				if resp.Object != "response" {
					t.Errorf("Object = %q, want %q", resp.Object, "response")
				}
				if resp.Model != "gpt-4o" {
					t.Errorf("Model = %q, want %q", resp.Model, "gpt-4o")
				}
				if resp.Status != "completed" {
					t.Errorf("Status = %q, want %q", resp.Status, "completed")
				}
				if len(resp.Output) != 1 {
					t.Fatalf("len(Output) = %d, want 1", len(resp.Output))
				}
				if len(resp.Output[0].Content) != 1 {
					t.Fatalf("len(Output[0].Content) = %d, want 1", len(resp.Output[0].Content))
				}
				if resp.Output[0].Content[0].Text != "Hello! How can I help you today?" {
					t.Errorf("Output text = %q, want %q", resp.Output[0].Content[0].Text, "Hello! How can I help you today?")
				}
				if resp.Usage == nil {
					t.Fatal("Usage should not be nil")
				}
				if resp.Usage.InputTokens != 10 {
					t.Errorf("InputTokens = %d, want 10", resp.Usage.InputTokens)
				}
				if resp.Usage.OutputTokens != 20 {
					t.Errorf("OutputTokens = %d, want 20", resp.Usage.OutputTokens)
				}
				if resp.Usage.TotalTokens != 30 {
					t.Errorf("TotalTokens = %d, want 30", resp.Usage.TotalTokens)
				}
			},
		},
		{
			name:          "API error - unauthorized",
			statusCode:    http.StatusUnauthorized,
			responseBody:  `{"error": {"message": "Invalid API key"}}`,
			expectedError: true,
		},
		{
			name:          "rate limit error",
			statusCode:    http.StatusTooManyRequests,
			responseBody:  `{"error": {"message": "Rate limit exceeded"}}`,
			expectedError: true,
		},
		{
			name:          "server error",
			statusCode:    http.StatusInternalServerError,
			responseBody:  `{"error": {"message": "Internal server error"}}`,
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
				authHeader := r.Header.Get("Authorization")
				if !strings.HasPrefix(authHeader, "Bearer ") {
					t.Errorf("Authorization header should start with 'Bearer '")
				}

				// Verify request path
				if r.URL.Path != "/responses" {
					t.Errorf("Path = %q, want %q", r.URL.Path, "/responses")
				}

				// Verify request body
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("failed to read request body: %v", err)
				}
				var req core.ResponsesRequest
				if err := json.Unmarshal(body, &req); err != nil {
					t.Fatalf("failed to unmarshal request: %v", err)
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			provider := New("test-api-key")
			provider.baseURL = server.URL

			req := &core.ResponsesRequest{
				Model: "gpt-4o",
				Input: "Hello",
			}

			resp, err := provider.Responses(context.Background(), req)

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

func TestResponsesWithArrayInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request body contains array input
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}

		// Verify input is an array
		input, ok := req["input"].([]interface{})
		if !ok {
			t.Fatal("input should be an array")
		}
		if len(input) != 2 {
			t.Errorf("len(input) = %d, want 2", len(input))
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "resp_123",
			"object": "response",
			"created_at": 1677652288,
			"model": "gpt-4o",
			"status": "completed",
			"output": [{
				"id": "msg_123",
				"type": "message",
				"role": "assistant",
				"status": "completed",
				"content": [{
					"type": "output_text",
					"text": "Hello!"
				}]
			}]
		}`))
	}))
	defer server.Close()

	provider := New("test-api-key")
	provider.baseURL = server.URL

	req := &core.ResponsesRequest{
		Model: "gpt-4o",
		Input: []interface{}{
			map[string]interface{}{
				"role":    "user",
				"content": "Hello",
			},
			map[string]interface{}{
				"role":    "assistant",
				"content": "Hi there!",
			},
		},
		Instructions: "Be helpful",
	}

	resp, err := provider.Responses(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ID != "resp_123" {
		t.Errorf("ID = %q, want %q", resp.ID, "resp_123")
	}
}

func TestStreamResponses(t *testing.T) {
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
			responseBody: `event: response.created
data: {"type":"response.created","response":{"id":"resp_123","object":"response","status":"in_progress","model":"gpt-4o"}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"Hello"}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"!"}

event: response.done
data: {"type":"response.done","response":{"id":"resp_123","object":"response","status":"completed","model":"gpt-4o"}}

data: [DONE]
`,
			expectedError: false,
			checkStream: func(t *testing.T, body io.ReadCloser) {
				if body == nil {
					t.Fatal("body should not be nil")
				}
				defer func() { _ = body.Close() }()

				// Read and verify the streaming response
				respBody, err := io.ReadAll(body)
				if err != nil {
					t.Fatalf("failed to read response body: %v", err)
				}

				responseStr := string(respBody)
				if !strings.Contains(responseStr, "response.created") {
					t.Error("response should contain response.created event")
				}
				if !strings.Contains(responseStr, "response.output_text.delta") {
					t.Error("response should contain response.output_text.delta event")
				}
				if !strings.Contains(responseStr, "[DONE]") {
					t.Error("response should end with [DONE]")
				}
			},
		},
		{
			name:          "API error - unauthorized",
			statusCode:    http.StatusUnauthorized,
			responseBody:  `{"error": {"message": "Invalid API key"}}`,
			expectedError: true,
		},
		{
			name:          "rate limit error",
			statusCode:    http.StatusTooManyRequests,
			responseBody:  `{"error": {"message": "Rate limit exceeded"}}`,
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
				authHeader := r.Header.Get("Authorization")
				if !strings.HasPrefix(authHeader, "Bearer ") {
					t.Errorf("Authorization header should start with 'Bearer '")
				}

				// Verify request path
				if r.URL.Path != "/responses" {
					t.Errorf("Path = %q, want %q", r.URL.Path, "/responses")
				}

				// Verify stream is set in request body
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("failed to read request body: %v", err)
				}
				var req core.ResponsesRequest
				if err := json.Unmarshal(body, &req); err != nil {
					t.Fatalf("failed to unmarshal request: %v", err)
				}
				if !req.Stream {
					t.Error("Stream should be true in request")
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			provider := New("test-api-key")
			provider.baseURL = server.URL

			req := &core.ResponsesRequest{
				Model: "gpt-4o",
				Input: "Hello",
			}

			body, err := provider.StreamResponses(context.Background(), req)

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

func TestResponsesWithContext(t *testing.T) {
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

	req := &core.ResponsesRequest{
		Model: "gpt-4o",
		Input: "Hello",
	}

	_, err := provider.Responses(ctx, req)
	if err == nil {
		t.Error("expected error when context is cancelled, got nil")
	}
}
