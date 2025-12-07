// Package core provides core types and interfaces for the LLM gateway.
package core

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ErrorType represents the type of error that occurred
type ErrorType string

const (
	// ErrorTypeProvider indicates an upstream provider error (5xx)
	ErrorTypeProvider ErrorType = "provider_error"
	// ErrorTypeRateLimit indicates a rate limit error (429)
	ErrorTypeRateLimit ErrorType = "rate_limit_error"
	// ErrorTypeInvalidRequest indicates a client error (4xx)
	ErrorTypeInvalidRequest ErrorType = "invalid_request_error"
	// ErrorTypeAuthentication indicates an authentication error (401)
	ErrorTypeAuthentication ErrorType = "authentication_error"
	// ErrorTypeNotFound indicates a not found error (404)
	ErrorTypeNotFound ErrorType = "not_found_error"
)

// GatewayError is the base error type for all gateway errors
type GatewayError struct {
	Type       ErrorType `json:"type"`
	Message    string    `json:"message"`
	StatusCode int       `json:"status_code"`
	Provider   string    `json:"provider,omitempty"`
	// Original error for debugging (not exposed to clients)
	Err error `json:"-"`
}

// Error implements the error interface
func (e *GatewayError) Error() string {
	if e.Provider != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Provider, e.Type, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap implements the error unwrapping interface
func (e *GatewayError) Unwrap() error {
	return e.Err
}

// HTTPStatusCode returns the appropriate HTTP status code for this error
func (e *GatewayError) HTTPStatusCode() int {
	if e.StatusCode != 0 {
		return e.StatusCode
	}
	// Default status codes based on error type
	switch e.Type {
	case ErrorTypeRateLimit:
		return http.StatusTooManyRequests
	case ErrorTypeInvalidRequest:
		return http.StatusBadRequest
	case ErrorTypeAuthentication:
		return http.StatusUnauthorized
	case ErrorTypeNotFound:
		return http.StatusNotFound
	case ErrorTypeProvider:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// ToJSON converts the error to a JSON-compatible map
func (e *GatewayError) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"error": map[string]interface{}{
			"type":    e.Type,
			"message": e.Message,
		},
	}
}

// NewProviderError creates a new provider error (upstream 5xx)
func NewProviderError(provider string, statusCode int, message string, err error) *GatewayError {
	return &GatewayError{
		Type:       ErrorTypeProvider,
		Message:    message,
		StatusCode: statusCode,
		Provider:   provider,
		Err:        err,
	}
}

// NewRateLimitError creates a new rate limit error (429)
func NewRateLimitError(provider string, message string) *GatewayError {
	return &GatewayError{
		Type:       ErrorTypeRateLimit,
		Message:    message,
		StatusCode: http.StatusTooManyRequests,
		Provider:   provider,
	}
}

// NewInvalidRequestError creates a new invalid request error (400)
func NewInvalidRequestError(message string, err error) *GatewayError {
	return NewInvalidRequestErrorWithStatus(http.StatusBadRequest, message, err)
}

// NewInvalidRequestErrorWithStatus creates a new invalid request error with a specific status code
func NewInvalidRequestErrorWithStatus(statusCode int, message string, err error) *GatewayError {
	return &GatewayError{
		Type:       ErrorTypeInvalidRequest,
		Message:    message,
		StatusCode: statusCode,
		Err:        err,
	}
}

// NewAuthenticationError creates a new authentication error (401)
func NewAuthenticationError(provider string, message string) *GatewayError {
	return &GatewayError{
		Type:       ErrorTypeAuthentication,
		Message:    message,
		StatusCode: http.StatusUnauthorized,
		Provider:   provider,
	}
}

// NewNotFoundError creates a new not found error (404)
func NewNotFoundError(message string) *GatewayError {
	return &GatewayError{
		Type:       ErrorTypeNotFound,
		Message:    message,
		StatusCode: http.StatusNotFound,
	}
}

// ParseProviderError parses an error response from a provider and returns an appropriate GatewayError
func ParseProviderError(provider string, statusCode int, body []byte, originalErr error) *GatewayError {
	// Try to parse the error response as JSON
	var errorResponse struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	message := string(body)
	if err := json.Unmarshal(body, &errorResponse); err == nil && errorResponse.Error.Message != "" {
		message = errorResponse.Error.Message
	}

	// Determine error type based on status code
	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return NewAuthenticationError(provider, message)
	case statusCode == http.StatusTooManyRequests:
		return NewRateLimitError(provider, message)
	case statusCode >= 400 && statusCode < 500:
		// Client errors from provider - mark as invalid request and preserve both provider info and original status code
		err := NewInvalidRequestErrorWithStatus(statusCode, message, originalErr)
		err.Provider = provider
		return err
	case statusCode >= 500:
		return NewProviderError(provider, http.StatusBadGateway, message, originalErr)
	default:
		return NewProviderError(provider, http.StatusBadGateway, message, originalErr)
	}
}

