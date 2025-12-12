package llmclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gomodel/internal/core"
)

func TestClient_Do_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"hello"}`))
	}))
	defer server.Close()

	client := New(
		DefaultConfig("test", server.URL),
		func(req *http.Request) {
			req.Header.Set("X-Test", "value")
		},
	)

	var result struct {
		Message string `json:"message"`
	}
	err := client.Do(context.Background(), Request{
		Method:   http.MethodGet,
		Endpoint: "/test",
	}, &result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Message != "hello" {
		t.Errorf("expected message 'hello', got '%s'", result.Message)
	}
}

func TestClient_Do_WithRequestBody(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got '%s'", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := New(DefaultConfig("test", server.URL), nil)

	requestBody := map[string]string{"input": "test"}
	var result map[string]string
	err := client.Do(context.Background(), Request{
		Method:   http.MethodPost,
		Endpoint: "/test",
		Body:     requestBody,
	}, &result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedBody["input"] != "test" {
		t.Errorf("expected input 'test', got '%v'", receivedBody["input"])
	}
}

func TestClient_Do_Headers(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := New(
		DefaultConfig("test", server.URL),
		func(req *http.Request) {
			req.Header.Set("Authorization", "Bearer token")
		},
	)

	err := client.Do(context.Background(), Request{
		Method:   http.MethodGet,
		Endpoint: "/test",
		Headers: map[string]string{
			"X-Custom": "custom-value",
		},
	}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedHeaders.Get("Authorization") != "Bearer token" {
		t.Errorf("expected Authorization header 'Bearer token', got '%s'", receivedHeaders.Get("Authorization"))
	}
	if receivedHeaders.Get("X-Custom") != "custom-value" {
		t.Errorf("expected X-Custom header 'custom-value', got '%s'", receivedHeaders.Get("X-Custom"))
	}
}

func TestClient_Do_ErrorParsing(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantType   core.ErrorType
	}{
		{
			name:       "rate limit",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":{"message":"Rate limited"}}`,
			wantType:   core.ErrorTypeRateLimit,
		},
		{
			name:       "authentication",
			statusCode: http.StatusUnauthorized,
			body:       `{"error":{"message":"Invalid API key"}}`,
			wantType:   core.ErrorTypeAuthentication,
		},
		{
			name:       "bad request",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"message":"Invalid model"}}`,
			wantType:   core.ErrorTypeInvalidRequest,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"error":{"message":"Server error"}}`,
			wantType:   core.ErrorTypeProvider,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			config := DefaultConfig("test", server.URL)
			config.MaxRetries = 0 // No retries for this test
			client := New(config, nil)

			err := client.Do(context.Background(), Request{
				Method:   http.MethodGet,
				Endpoint: "/test",
			}, nil)

			if err == nil {
				t.Fatal("expected error, got nil")
			}
			gatewayErr, ok := err.(*core.GatewayError)
			if !ok {
				t.Fatalf("expected GatewayError, got %T", err)
			}
			if gatewayErr.Type != tt.wantType {
				t.Errorf("expected error type %s, got %s", tt.wantType, gatewayErr.Type)
			}
		})
	}
}

func TestClient_Do_Retries(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"Rate limited"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	config := DefaultConfig("test", server.URL)
	config.MaxRetries = 3
	config.InitialBackoff = 10 * time.Millisecond // Fast backoff for tests
	client := New(config, nil)

	var result struct {
		Success bool `json:"success"`
	}
	err := client.Do(context.Background(), Request{
		Method:   http.MethodGet,
		Endpoint: "/test",
	}, &result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success to be true")
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestClient_Do_RetriesExhausted(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"Rate limited"}}`))
	}))
	defer server.Close()

	config := DefaultConfig("test", server.URL)
	config.MaxRetries = 2
	config.InitialBackoff = 10 * time.Millisecond
	client := New(config, nil)

	err := client.Do(context.Background(), Request{
		Method:   http.MethodGet,
		Endpoint: "/test",
	}, nil)

	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
	// 1 initial + 2 retries = 3 attempts
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestClient_DoStream_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"chunk\":1}\n\n"))
		_, _ = w.Write([]byte("data: {\"chunk\":2}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := New(DefaultConfig("test", server.URL), nil)

	stream, err := client.DoStream(context.Background(), Request{
		Method:   http.MethodPost,
		Endpoint: "/stream",
		Body:     map[string]bool{"stream": true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer stream.Close()

	body, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("failed to read stream: %v", err)
	}

	if !strings.Contains(string(body), "chunk") {
		t.Errorf("expected body to contain 'chunk', got: %s", string(body))
	}
}

func TestClient_DoStream_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid API key"}}`))
	}))
	defer server.Close()

	client := New(DefaultConfig("test", server.URL), nil)

	_, err := client.DoStream(context.Background(), Request{
		Method:   http.MethodPost,
		Endpoint: "/stream",
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	gatewayErr, ok := err.(*core.GatewayError)
	if !ok {
		t.Fatalf("expected GatewayError, got %T", err)
	}
	if gatewayErr.Type != core.ErrorTypeAuthentication {
		t.Errorf("expected error type %s, got %s", core.ErrorTypeAuthentication, gatewayErr.Type)
	}
}

func TestCircuitBreaker_OpensAfterFailures(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"Server error"}}`))
	}))
	defer server.Close()

	config := DefaultConfig("test", server.URL)
	config.MaxRetries = 0 // No retries
	config.CircuitBreaker = &CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
	}
	client := New(config, nil)

	// Make requests until circuit opens
	for i := 0; i < 5; i++ {
		_ = client.Do(context.Background(), Request{
			Method:   http.MethodGet,
			Endpoint: "/test",
		}, nil)
	}

	// Circuit should be open now - requests should fail immediately
	err := client.Do(context.Background(), Request{
		Method:   http.MethodGet,
		Endpoint: "/test",
	}, nil)

	if err == nil {
		t.Fatal("expected circuit breaker error")
	}
	gatewayErr, ok := err.(*core.GatewayError)
	if !ok {
		t.Fatalf("expected GatewayError, got %T", err)
	}
	if gatewayErr.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, gatewayErr.StatusCode)
	}
	if !strings.Contains(gatewayErr.Message, "circuit breaker") {
		t.Errorf("expected circuit breaker message, got: %s", gatewayErr.Message)
	}

	// Should have made exactly 3 requests (threshold)
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts before circuit opened, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestCircuitBreaker_ClosesAfterTimeout(t *testing.T) {
	var attempts int32
	var shouldSucceed bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		if shouldSucceed {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"Server error"}}`))
	}))
	defer server.Close()

	config := DefaultConfig("test", server.URL)
	config.MaxRetries = 0
	config.CircuitBreaker = &CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond, // Short timeout for testing
	}
	client := New(config, nil)

	// Trigger circuit breaker to open
	for i := 0; i < 2; i++ {
		_ = client.Do(context.Background(), Request{
			Method:   http.MethodGet,
			Endpoint: "/test",
		}, nil)
	}

	// Verify circuit is open
	err := client.Do(context.Background(), Request{
		Method:   http.MethodGet,
		Endpoint: "/test",
	}, nil)
	if err == nil {
		t.Fatal("expected circuit to be open")
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Now make server succeed
	shouldSucceed = true

	// Should be able to make request (half-open state)
	var result struct {
		Success bool `json:"success"`
	}
	err = client.Do(context.Background(), Request{
		Method:   http.MethodGet,
		Endpoint: "/test",
	}, &result)

	if err != nil {
		t.Fatalf("expected success after timeout, got: %v", err)
	}
	if !result.Success {
		t.Error("expected success to be true")
	}
}

func TestCircuitBreaker_State(t *testing.T) {
	cb := newCircuitBreaker(3, 2, time.Minute)

	if state := cb.State(); state != "closed" {
		t.Errorf("expected initial state 'closed', got '%s'", state)
	}

	// Record failures to open circuit
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	if state := cb.State(); state != "open" {
		t.Errorf("expected state 'open' after failures, got '%s'", state)
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := New(DefaultConfig("test", server.URL), nil)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := client.Do(ctx, Request{
		Method:   http.MethodGet,
		Endpoint: "/test",
	}, nil)

	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig("test-provider", "https://api.test.com")

	if config.ProviderName != "test-provider" {
		t.Errorf("expected provider name 'test-provider', got '%s'", config.ProviderName)
	}
	if config.BaseURL != "https://api.test.com" {
		t.Errorf("expected base URL 'https://api.test.com', got '%s'", config.BaseURL)
	}
	if config.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", config.MaxRetries)
	}
	if config.InitialBackoff != 1*time.Second {
		t.Errorf("expected InitialBackoff 1s, got %v", config.InitialBackoff)
	}
	if config.CircuitBreaker == nil {
		t.Error("expected CircuitBreaker config to be set")
	}
}

func TestClient_SetBaseURL(t *testing.T) {
	client := New(DefaultConfig("test", "https://original.com"), nil)

	if client.BaseURL() != "https://original.com" {
		t.Errorf("expected base URL 'https://original.com', got '%s'", client.BaseURL())
	}

	client.SetBaseURL("https://new.com")

	if client.BaseURL() != "https://new.com" {
		t.Errorf("expected base URL 'https://new.com', got '%s'", client.BaseURL())
	}
}

func TestClient_NonRetryableErrors(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Bad request"}}`))
	}))
	defer server.Close()

	config := DefaultConfig("test", server.URL)
	config.MaxRetries = 3
	client := New(config, nil)

	err := client.Do(context.Background(), Request{
		Method:   http.MethodGet,
		Endpoint: "/test",
	}, nil)

	if err == nil {
		t.Fatal("expected error")
	}
	// Should NOT retry on 400 errors
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected 1 attempt (no retries on 400), got %d", atomic.LoadInt32(&attempts))
	}
}

func TestBackoffCalculation(t *testing.T) {
	config := DefaultConfig("test", "http://test.com")
	config.InitialBackoff = 100 * time.Millisecond
	config.MaxBackoff = 1 * time.Second
	config.BackoffFactor = 2.0
	client := New(config, nil)

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 100 * time.Millisecond}, // Initial
		{2, 200 * time.Millisecond}, // 100 * 2
		{3, 400 * time.Millisecond}, // 100 * 4
		{4, 800 * time.Millisecond}, // 100 * 8
		{5, 1 * time.Second},        // Capped at max
		{10, 1 * time.Second},       // Still capped
	}

	for _, tt := range tests {
		result := client.calculateBackoff(tt.attempt)
		if result != tt.expected {
			t.Errorf("attempt %d: expected backoff %v, got %v", tt.attempt, tt.expected, result)
		}
	}
}
