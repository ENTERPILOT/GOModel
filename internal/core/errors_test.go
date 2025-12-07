package core

import (
	"errors"
	"net/http"
	"testing"
)

func TestGatewayError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *GatewayError
		expected string
	}{
		{
			name: "error with provider",
			err: &GatewayError{
				Type:     ErrorTypeProvider,
				Message:  "upstream error",
				Provider: "openai",
			},
			expected: "[openai] provider_error: upstream error",
		},
		{
			name: "error without provider",
			err: &GatewayError{
				Type:    ErrorTypeInvalidRequest,
				Message: "bad request",
			},
			expected: "invalid_request_error: bad request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGatewayError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	gatewayErr := &GatewayError{
		Type:    ErrorTypeProvider,
		Message: "wrapped error",
		Err:     originalErr,
	}

	if unwrapped := gatewayErr.Unwrap(); unwrapped != originalErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, originalErr)
	}
}

func TestGatewayError_HTTPStatusCode(t *testing.T) {
	tests := []struct {
		name     string
		err      *GatewayError
		expected int
	}{
		{
			name: "explicit status code",
			err: &GatewayError{
				Type:       ErrorTypeProvider,
				StatusCode: http.StatusServiceUnavailable,
			},
			expected: http.StatusServiceUnavailable,
		},
		{
			name: "rate limit default",
			err: &GatewayError{
				Type: ErrorTypeRateLimit,
			},
			expected: http.StatusTooManyRequests,
		},
		{
			name: "invalid request default",
			err: &GatewayError{
				Type: ErrorTypeInvalidRequest,
			},
			expected: http.StatusBadRequest,
		},
		{
			name: "authentication default",
			err: &GatewayError{
				Type: ErrorTypeAuthentication,
			},
			expected: http.StatusUnauthorized,
		},
		{
			name: "not found default",
			err: &GatewayError{
				Type: ErrorTypeNotFound,
			},
			expected: http.StatusNotFound,
		},
		{
			name: "provider error default",
			err: &GatewayError{
				Type: ErrorTypeProvider,
			},
			expected: http.StatusBadGateway,
		},
		{
			name: "unknown error type",
			err: &GatewayError{
				Type: ErrorType("unknown"),
			},
			expected: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.HTTPStatusCode(); got != tt.expected {
				t.Errorf("HTTPStatusCode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGatewayError_ToJSON(t *testing.T) {
	err := &GatewayError{
		Type:    ErrorTypeRateLimit,
		Message: "too many requests",
	}

	result := err.ToJSON()

	errorData, ok := result["error"].(map[string]interface{})
	if !ok {
		t.Fatal("ToJSON() should return map with 'error' key")
	}

	if errorData["type"] != ErrorTypeRateLimit {
		t.Errorf("ToJSON() type = %v, want %v", errorData["type"], ErrorTypeRateLimit)
	}

	if errorData["message"] != "too many requests" {
		t.Errorf("ToJSON() message = %v, want %v", errorData["message"], "too many requests")
	}
}

func TestNewProviderError(t *testing.T) {
	originalErr := errors.New("connection failed")
	err := NewProviderError("openai", http.StatusBadGateway, "upstream failed", originalErr)

	if err.Type != ErrorTypeProvider {
		t.Errorf("Type = %v, want %v", err.Type, ErrorTypeProvider)
	}

	if err.Provider != "openai" {
		t.Errorf("Provider = %v, want %v", err.Provider, "openai")
	}

	if err.StatusCode != http.StatusBadGateway {
		t.Errorf("StatusCode = %v, want %v", err.StatusCode, http.StatusBadGateway)
	}

	if err.Message != "upstream failed" {
		t.Errorf("Message = %v, want %v", err.Message, "upstream failed")
	}

	if err.Err != originalErr {
		t.Errorf("Err = %v, want %v", err.Err, originalErr)
	}
}

func TestNewRateLimitError(t *testing.T) {
	err := NewRateLimitError("anthropic", "rate limit exceeded")

	if err.Type != ErrorTypeRateLimit {
		t.Errorf("Type = %v, want %v", err.Type, ErrorTypeRateLimit)
	}

	if err.Provider != "anthropic" {
		t.Errorf("Provider = %v, want %v", err.Provider, "anthropic")
	}

	if err.StatusCode != http.StatusTooManyRequests {
		t.Errorf("StatusCode = %v, want %v", err.StatusCode, http.StatusTooManyRequests)
	}

	if err.Message != "rate limit exceeded" {
		t.Errorf("Message = %v, want %v", err.Message, "rate limit exceeded")
	}
}

func TestNewInvalidRequestError(t *testing.T) {
	originalErr := errors.New("missing field")
	err := NewInvalidRequestError("invalid input", originalErr)

	if err.Type != ErrorTypeInvalidRequest {
		t.Errorf("Type = %v, want %v", err.Type, ErrorTypeInvalidRequest)
	}

	if err.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %v, want %v", err.StatusCode, http.StatusBadRequest)
	}

	if err.Message != "invalid input" {
		t.Errorf("Message = %v, want %v", err.Message, "invalid input")
	}

	if err.Err != originalErr {
		t.Errorf("Err = %v, want %v", err.Err, originalErr)
	}
}

func TestNewAuthenticationError(t *testing.T) {
	err := NewAuthenticationError("gemini", "invalid API key")

	if err.Type != ErrorTypeAuthentication {
		t.Errorf("Type = %v, want %v", err.Type, ErrorTypeAuthentication)
	}

	if err.Provider != "gemini" {
		t.Errorf("Provider = %v, want %v", err.Provider, "gemini")
	}

	if err.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %v, want %v", err.StatusCode, http.StatusUnauthorized)
	}

	if err.Message != "invalid API key" {
		t.Errorf("Message = %v, want %v", err.Message, "invalid API key")
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("model not found")

	if err.Type != ErrorTypeNotFound {
		t.Errorf("Type = %v, want %v", err.Type, ErrorTypeNotFound)
	}

	if err.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %v, want %v", err.StatusCode, http.StatusNotFound)
	}

	if err.Message != "model not found" {
		t.Errorf("Message = %v, want %v", err.Message, "model not found")
	}
}

func TestParseProviderError(t *testing.T) {
	tests := []struct {
		name           string
		provider       string
		statusCode     int
		body           []byte
		expectedType   ErrorType
		expectedStatus int
	}{
		{
			name:           "401 unauthorized",
			provider:       "openai",
			statusCode:     http.StatusUnauthorized,
			body:           []byte(`{"error": {"message": "Invalid API key"}}`),
			expectedType:   ErrorTypeAuthentication,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "403 forbidden",
			provider:       "anthropic",
			statusCode:     http.StatusForbidden,
			body:           []byte(`{"error": {"message": "Access denied"}}`),
			expectedType:   ErrorTypeAuthentication,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "429 rate limit",
			provider:       "gemini",
			statusCode:     http.StatusTooManyRequests,
			body:           []byte(`{"error": {"message": "Rate limit exceeded"}}`),
			expectedType:   ErrorTypeRateLimit,
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			name:           "400 bad request",
			provider:       "openai",
			statusCode:     http.StatusBadRequest,
			body:           []byte(`{"error": {"message": "Invalid parameters"}}`),
			expectedType:   ErrorTypeInvalidRequest,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "500 server error",
			provider:       "anthropic",
			statusCode:     http.StatusInternalServerError,
			body:           []byte(`{"error": {"message": "Internal server error"}}`),
			expectedType:   ErrorTypeProvider,
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:           "502 bad gateway",
			provider:       "gemini",
			statusCode:     http.StatusBadGateway,
			body:           []byte(`{"error": {"message": "Bad gateway"}}`),
			expectedType:   ErrorTypeProvider,
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:           "plain text error response",
			provider:       "openai",
			statusCode:     http.StatusInternalServerError,
			body:           []byte("Internal Server Error"),
			expectedType:   ErrorTypeProvider,
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:           "json parse with message",
			provider:       "openai",
			statusCode:     http.StatusBadRequest,
			body:           []byte(`{"error": {"message": "Model not found", "type": "not_found"}}`),
			expectedType:   ErrorTypeInvalidRequest,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ParseProviderError(tt.provider, tt.statusCode, tt.body, nil)

			if err.Type != tt.expectedType {
				t.Errorf("Type = %v, want %v", err.Type, tt.expectedType)
			}

			if err.HTTPStatusCode() != tt.expectedStatus {
				t.Errorf("HTTPStatusCode() = %v, want %v", err.HTTPStatusCode(), tt.expectedStatus)
			}

			if err.Provider != tt.provider {
				t.Errorf("Provider = %v, want %v", err.Provider, tt.provider)
			}

			if err.Message == "" {
				t.Error("Message should not be empty")
			}
		})
	}
}

func TestGatewayError_AsError(t *testing.T) {
	// Test that GatewayError can be used with errors.As
	originalErr := NewRateLimitError("openai", "too many requests")
	var err error = originalErr

	var gatewayErr *GatewayError
	if !errors.As(err, &gatewayErr) {
		t.Error("errors.As should work with GatewayError")
	}

	if gatewayErr.Type != ErrorTypeRateLimit {
		t.Errorf("Type = %v, want %v", gatewayErr.Type, ErrorTypeRateLimit)
	}
}

func TestGatewayError_IsError(t *testing.T) {
	// Test that GatewayError can be used with errors.Is
	originalErr := errors.New("network error")
	gatewayErr := NewProviderError("openai", http.StatusBadGateway, "connection failed", originalErr)

	if !errors.Is(gatewayErr, originalErr) {
		t.Error("errors.Is should work with wrapped errors in GatewayError")
	}
}

