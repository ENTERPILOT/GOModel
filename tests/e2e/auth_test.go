//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gomodel/internal/core"
	"gomodel/internal/providers"
	"gomodel/internal/server"
)

const testMasterKey = "test-secret-key-12345"

// mockProviderForAuth is a simple mock provider for authentication tests
type mockProviderForAuth struct {
	models []core.Model
}

func (m *mockProviderForAuth) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	return &core.ChatResponse{
		ID:      "mock-chat-id",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   req.Model,
		Choices: []core.Choice{
			{
				Index: 0,
				Message: core.Message{
					Role:    "assistant",
					Content: "Mock response",
				},
				FinishReason: "stop",
			},
		},
	}, nil
}

func (m *mockProviderForAuth) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	// Return a simple SSE stream
	data := `data: {"id":"mock-stream","object":"chat.completion.chunk","created":1234567890,"model":"` + req.Model + `","choices":[{"index":0,"delta":{"content":"test"},"finish_reason":null}]}

data: [DONE]
`
	return io.NopCloser(bytes.NewBufferString(data)), nil
}

func (m *mockProviderForAuth) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	return &core.ModelsResponse{
		Object: "list",
		Data:   m.models,
	}, nil
}

func (m *mockProviderForAuth) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	return &core.ResponsesResponse{
		ID:        "mock-responses-id",
		Object:    "response",
		CreatedAt: 1234567890,
		Model:     req.Model,
		Status:    "completed",
		Output: []core.ResponsesOutputItem{
			{
				ID:   "msg-1",
				Type: "message",
				Role: "assistant",
				Content: []core.ResponsesContentItem{
					{
						Type: "text",
						Text: "Mock response",
					},
				},
			},
		},
	}, nil
}

func (m *mockProviderForAuth) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	// Return a simple SSE stream
	data := `data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"test"}}

data: [DONE]
`
	return io.NopCloser(bytes.NewBufferString(data)), nil
}

// setupAuthServer creates a new server instance with authentication enabled
func setupAuthServer(t *testing.T, masterKey string) *server.Server {
	t.Helper()

	// Create a mock provider
	mockProvider := &mockProviderForAuth{
		models: []core.Model{
			{ID: "mock-model-1", Object: "model", OwnedBy: "mock"},
		},
	}

	// Create registry and register mock provider
	registry := providers.NewModelRegistry()
	registry.RegisterProvider(mockProvider)

	// Initialize registry synchronously for tests
	if err := registry.Initialize(context.Background()); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	// Create router
	router, err := providers.NewRouter(registry)
	if err != nil {
		t.Fatalf("Failed to create router: %v", err)
	}

	// Create server with master key
	cfg := &server.Config{
		MasterKey: masterKey,
	}
	return server.New(router, cfg)
}

func TestAuthenticationE2E(t *testing.T) {
	srv := setupAuthServer(t, testMasterKey)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	tests := []struct {
		name           string
		endpoint       string
		method         string
		authHeader     string
		body           interface{}
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:           "GET /health with valid auth",
			endpoint:       "/health",
			method:         http.MethodGet,
			authHeader:     "Bearer " + testMasterKey,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]string
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, "ok", resp["status"])
			},
		},
		{
			name:           "GET /health without auth - public endpoint",
			endpoint:       "/health",
			method:         http.MethodGet,
			authHeader:     "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				// Health endpoint is public for load balancer health checks
				var resp map[string]string
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, "ok", resp["status"])
			},
		},
		{
			name:           "GET /health with invalid auth - still works (public)",
			endpoint:       "/health",
			method:         http.MethodGet,
			authHeader:     "Bearer wrong-key",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				// Health endpoint is public, ignores auth headers
				var resp map[string]string
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, "ok", resp["status"])
			},
		},
		{
			name:           "GET /v1/models with malformed auth header",
			endpoint:       "/v1/models",
			method:         http.MethodGet,
			authHeader:     testMasterKey,
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errMap := resp["error"].(map[string]interface{})
				assert.Equal(t, "authentication_error", errMap["type"])
				assert.Contains(t, errMap["message"], "invalid authorization header format")
			},
		},
		{
			name:           "GET /v1/models with valid auth",
			endpoint:       "/v1/models",
			method:         http.MethodGet,
			authHeader:     "Bearer " + testMasterKey,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp core.ModelsResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.Equal(t, "list", resp.Object)
				assert.NotEmpty(t, resp.Data)
			},
		},
		{
			name:           "GET /v1/models without auth",
			endpoint:       "/v1/models",
			method:         http.MethodGet,
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errMap := resp["error"].(map[string]interface{})
				assert.Equal(t, "authentication_error", errMap["type"])
			},
		},
		{
			name:       "POST /v1/chat/completions with valid auth",
			endpoint:   "/v1/chat/completions",
			method:     http.MethodPost,
			authHeader: "Bearer " + testMasterKey,
			body: map[string]interface{}{
				"model": "mock-model-1",
				"messages": []map[string]string{
					{"role": "user", "content": "Hello"},
				},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp core.ChatResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.NotEmpty(t, resp.ID)
				assert.Equal(t, "chat.completion", resp.Object)
			},
		},
		{
			name:       "POST /v1/chat/completions without auth",
			endpoint:   "/v1/chat/completions",
			method:     http.MethodPost,
			authHeader: "",
			body: map[string]interface{}{
				"model": "mock-model-1",
				"messages": []map[string]string{
					{"role": "user", "content": "Hello"},
				},
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errMap := resp["error"].(map[string]interface{})
				assert.Equal(t, "authentication_error", errMap["type"])
			},
		},
		{
			name:       "POST /v1/responses with valid auth",
			endpoint:   "/v1/responses",
			method:     http.MethodPost,
			authHeader: "Bearer " + testMasterKey,
			body: map[string]interface{}{
				"model": "mock-model-1",
				"input": "Hello",
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var resp core.ResponsesResponse
				require.NoError(t, json.Unmarshal(body, &resp))
				assert.NotEmpty(t, resp.ID)
			},
		},
		{
			name:       "POST /v1/responses without auth",
			endpoint:   "/v1/responses",
			method:     http.MethodPost,
			authHeader: "",
			body: map[string]interface{}{
				"model": "mock-model-1",
				"input": "Hello",
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))
				errMap := resp["error"].(map[string]interface{})
				assert.Equal(t, "authentication_error", errMap["type"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reqBody io.Reader
			if tt.body != nil {
				bodyBytes, err := json.Marshal(tt.body)
				require.NoError(t, err)
				reqBody = bytes.NewReader(bodyBytes)
			}

			req, err := http.NewRequest(tt.method, ts.URL+tt.endpoint, reqBody)
			require.NoError(t, err)

			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			if tt.checkResponse != nil {
				tt.checkResponse(t, body)
			}
		})
	}
}

func TestAuthenticationDisabled(t *testing.T) {
	// Test server without master key (authentication disabled)
	srv := setupAuthServer(t, "")
	ts := httptest.NewServer(srv)
	defer ts.Close()

	tests := []struct {
		name           string
		endpoint       string
		method         string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "GET /health without auth works when auth disabled",
			endpoint:       "/health",
			method:         http.MethodGet,
			authHeader:     "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "GET /v1/models without auth works when auth disabled",
			endpoint:       "/v1/models",
			method:         http.MethodGet,
			authHeader:     "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "GET /health with auth header still works",
			endpoint:       "/health",
			method:         http.MethodGet,
			authHeader:     "Bearer some-random-key",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, ts.URL+tt.endpoint, nil)
			require.NoError(t, err)

			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestAuthenticationStreamingEndpoints(t *testing.T) {
	srv := setupAuthServer(t, testMasterKey)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	t.Run("streaming chat completion with valid auth", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model":  "mock-model-1",
			"stream": true,
			"messages": []map[string]string{
				{"role": "user", "content": "Hello"},
			},
		}

		bodyBytes, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/chat/completions", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+testMasterKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	})

	t.Run("streaming chat completion without auth", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model":  "mock-model-1",
			"stream": true,
			"messages": []map[string]string{
				{"role": "user", "content": "Hello"},
			},
		}

		bodyBytes, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/chat/completions", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var respBody map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		require.NoError(t, err)

		errMap := respBody["error"].(map[string]interface{})
		assert.Equal(t, "authentication_error", errMap["type"])
	})

	t.Run("streaming responses with valid auth", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model":  "mock-model-1",
			"stream": true,
			"input":  "Hello",
		}

		bodyBytes, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/responses", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+testMasterKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	})

	t.Run("streaming responses without auth", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model":  "mock-model-1",
			"stream": true,
			"input":  "Hello",
		}

		bodyBytes, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/responses", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var respBody map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		require.NoError(t, err)

		errMap := respBody["error"].(map[string]interface{})
		assert.Equal(t, "authentication_error", errMap["type"])
	})
}

// TestAuthenticationCaseSensitivity verifies that the master key is case-sensitive
func TestAuthenticationCaseSensitivity(t *testing.T) {
	srv := setupAuthServer(t, "MySecretKey123")
	ts := httptest.NewServer(srv)
	defer ts.Close()

	tests := []struct {
		name           string
		authKey        string
		expectedStatus int
	}{
		{
			name:           "exact match",
			authKey:        "MySecretKey123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "lowercase",
			authKey:        "mysecretkey123",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "uppercase",
			authKey:        "MYSECRETKEY123",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "mixed case",
			authKey:        "mySecretKey123",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use /v1/models instead of /health since /health is now public
			req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/models", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+tt.authKey)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

// TestAuthenticationWithSpecialCharacters verifies that master keys with special characters work
func TestAuthenticationWithSpecialCharacters(t *testing.T) {
	// Create a long key with printable characters
	longKey := "very-long-key-"
	for i := 0; i < 100; i++ {
		longKey += "x"
	}

	specialKeys := []string{
		"key-with-dashes",
		"key_with_underscores",
		"key.with.dots",
		"key$with$dollars",
		"key!@#$%^&*()",
		"key with spaces",
		longKey,
	}

	for i, key := range specialKeys {
		t.Run(fmt.Sprintf("special_key_%d", i), func(t *testing.T) {
			srv := setupAuthServer(t, key)
			ts := httptest.NewServer(srv)
			defer ts.Close()

			// Test with correct key - use /v1/models since /health is now public
			req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/models", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+key)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			// Test with incorrect key
			req, err = http.NewRequest(http.MethodGet, ts.URL+"/v1/models", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer wrong-key")

			resp, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

// TestAuthenticationBearerPrefixVariations verifies that only "Bearer " prefix works
func TestAuthenticationBearerPrefixVariations(t *testing.T) {
	srv := setupAuthServer(t, testMasterKey)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "correct Bearer prefix",
			authHeader:     "Bearer " + testMasterKey,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "lowercase bearer",
			authHeader:     "bearer " + testMasterKey,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "uppercase BEARER",
			authHeader:     "BEARER " + testMasterKey,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "no space after Bearer",
			authHeader:     "Bearer" + testMasterKey,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "double space",
			authHeader:     "Bearer  " + testMasterKey,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Basic auth instead",
			authHeader:     "Basic " + testMasterKey,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "no prefix",
			authHeader:     testMasterKey,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use /v1/models instead of /health since /health is now public
			req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/models", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", tt.authHeader)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}
