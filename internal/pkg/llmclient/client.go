// Package llmclient provides a base HTTP client for LLM providers with:
// - Request marshaling/unmarshaling
// - Retries with exponential backoff
// - Standardized error parsing (429, 5xx)
// - Circuit breaking
package llmclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"sync"
	"time"

	"gomodel/internal/core"
	"gomodel/internal/pkg/httpclient"
)

// Config holds configuration for the LLM client
type Config struct {
	// ProviderName identifies the provider for error messages
	ProviderName string

	// BaseURL is the API base URL
	BaseURL string

	// Retry configuration
	MaxRetries     int           // Maximum number of retry attempts (default: 3)
	InitialBackoff time.Duration // Initial backoff duration (default: 1s)
	MaxBackoff     time.Duration // Maximum backoff duration (default: 30s)
	BackoffFactor  float64       // Backoff multiplier (default: 2.0)

	// Circuit breaker configuration
	CircuitBreaker *CircuitBreakerConfig
}

// CircuitBreakerConfig holds circuit breaker settings
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of failures before opening the circuit
	FailureThreshold int
	// SuccessThreshold is the number of successes needed to close an open circuit
	SuccessThreshold int
	// Timeout is how long to wait before attempting to close an open circuit
	Timeout time.Duration
}

// DefaultConfig returns default client configuration
func DefaultConfig(providerName, baseURL string) Config {
	return Config{
		ProviderName:   providerName,
		BaseURL:        baseURL,
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		BackoffFactor:  2.0,
		CircuitBreaker: &CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 2,
			Timeout:          30 * time.Second,
		},
	}
}

// HeaderSetter is a function that sets headers on an HTTP request
type HeaderSetter func(req *http.Request)

// Client is a base HTTP client for LLM providers
type Client struct {
	httpClient     *http.Client
	config         Config
	headerSetter   HeaderSetter
	circuitBreaker *circuitBreaker
}

// New creates a new LLM client with the given configuration
func New(config Config, headerSetter HeaderSetter) *Client {
	c := &Client{
		httpClient:   httpclient.NewDefaultHTTPClient(),
		config:       config,
		headerSetter: headerSetter,
	}

	if config.CircuitBreaker != nil {
		c.circuitBreaker = newCircuitBreaker(
			config.CircuitBreaker.FailureThreshold,
			config.CircuitBreaker.SuccessThreshold,
			config.CircuitBreaker.Timeout,
		)
	}

	return c
}

// NewWithHTTPClient creates a new LLM client with a custom HTTP client
func NewWithHTTPClient(httpClient *http.Client, config Config, headerSetter HeaderSetter) *Client {
	c := &Client{
		httpClient:   httpClient,
		config:       config,
		headerSetter: headerSetter,
	}

	if config.CircuitBreaker != nil {
		c.circuitBreaker = newCircuitBreaker(
			config.CircuitBreaker.FailureThreshold,
			config.CircuitBreaker.SuccessThreshold,
			config.CircuitBreaker.Timeout,
		)
	}

	return c
}

// SetBaseURL updates the base URL
func (c *Client) SetBaseURL(url string) {
	c.config.BaseURL = url
}

// BaseURL returns the current base URL
func (c *Client) BaseURL() string {
	return c.config.BaseURL
}

// Request represents an HTTP request to be made
type Request struct {
	Method   string
	Endpoint string
	Body     interface{} // Will be JSON marshaled if not nil
	Headers  map[string]string
}

// Response represents an HTTP response
type Response struct {
	StatusCode int
	Body       []byte
}

// Do executes a request with retries and circuit breaking, then unmarshals the response
func (c *Client) Do(ctx context.Context, req Request, result interface{}) error {
	resp, err := c.DoRaw(ctx, req)
	if err != nil {
		return err
	}

	if result != nil {
		if err := json.Unmarshal(resp.Body, result); err != nil {
			return core.NewProviderError(c.config.ProviderName, http.StatusBadGateway, "failed to unmarshal response: "+err.Error(), err)
		}
	}

	return nil
}

// DoRaw executes a request with retries and circuit breaking, returning the raw response
func (c *Client) DoRaw(ctx context.Context, req Request) (*Response, error) {
	// Check circuit breaker
	if c.circuitBreaker != nil && !c.circuitBreaker.Allow() {
		return nil, core.NewProviderError(c.config.ProviderName, http.StatusServiceUnavailable,
			"circuit breaker is open - provider temporarily unavailable", nil)
	}

	var lastErr error
	maxAttempts := c.config.MaxRetries + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Calculate backoff duration with jitter
			backoff := c.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		resp, err := c.doRequest(ctx, req)
		if err != nil {
			lastErr = err
			// Only retry on network errors
			if c.circuitBreaker != nil {
				c.circuitBreaker.RecordFailure()
			}
			continue
		}

		// Check for retryable status codes
		if c.isRetryable(resp.StatusCode) {
			if c.circuitBreaker != nil {
				c.circuitBreaker.RecordFailure()
			}
			lastErr = core.ParseProviderError(c.config.ProviderName, resp.StatusCode, resp.Body, nil)
			continue
		}

		// Non-retryable error
		if resp.StatusCode != http.StatusOK {
			if c.circuitBreaker != nil {
				// Only record failure for server errors
				if resp.StatusCode >= 500 {
					c.circuitBreaker.RecordFailure()
				}
			}
			return nil, core.ParseProviderError(c.config.ProviderName, resp.StatusCode, resp.Body, nil)
		}

		// Success
		if c.circuitBreaker != nil {
			c.circuitBreaker.RecordSuccess()
		}
		return resp, nil
	}

	// All retries exhausted
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, core.NewProviderError(c.config.ProviderName, http.StatusBadGateway, "request failed after retries", nil)
}

// DoStream executes a streaming request, returning a ReadCloser
// Note: Streaming requests do NOT retry (as partial data may have been sent)
func (c *Client) DoStream(ctx context.Context, req Request) (io.ReadCloser, error) {
	// Check circuit breaker
	if c.circuitBreaker != nil && !c.circuitBreaker.Allow() {
		return nil, core.NewProviderError(c.config.ProviderName, http.StatusServiceUnavailable,
			"circuit breaker is open - provider temporarily unavailable", nil)
	}

	httpReq, err := c.buildRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if c.circuitBreaker != nil {
			c.circuitBreaker.RecordFailure()
		}
		return nil, core.NewProviderError(c.config.ProviderName, http.StatusBadGateway, "failed to send request: "+err.Error(), err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			respBody = []byte("failed to read error response")
		}
		_ = resp.Body.Close()

		if c.circuitBreaker != nil {
			if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
				c.circuitBreaker.RecordFailure()
			}
		}
		return nil, core.ParseProviderError(c.config.ProviderName, resp.StatusCode, respBody, nil)
	}

	if c.circuitBreaker != nil {
		c.circuitBreaker.RecordSuccess()
	}
	return resp.Body, nil
}

// doRequest executes a single HTTP request without retries
func (c *Client) doRequest(ctx context.Context, req Request) (*Response, error) {
	httpReq, err := c.buildRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, core.NewProviderError(c.config.ProviderName, http.StatusBadGateway, "failed to send request: "+err.Error(), err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, core.NewProviderError(c.config.ProviderName, http.StatusBadGateway, "failed to read response: "+err.Error(), err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       body,
	}, nil
}

// buildRequest creates an HTTP request from a Request
func (c *Client) buildRequest(ctx context.Context, req Request) (*http.Request, error) {
	url := c.config.BaseURL + req.Endpoint

	var bodyReader io.Reader
	if req.Body != nil {
		bodyBytes, err := json.Marshal(req.Body)
		if err != nil {
			return nil, core.NewInvalidRequestError("failed to marshal request", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, bodyReader)
	if err != nil {
		return nil, core.NewInvalidRequestError("failed to create request", err)
	}

	// Set default content type for requests with body
	if req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	// Apply provider-specific headers
	if c.headerSetter != nil {
		c.headerSetter(httpReq)
	}

	// Apply request-specific headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	return httpReq, nil
}

// calculateBackoff calculates the backoff duration for a given attempt
func (c *Client) calculateBackoff(attempt int) time.Duration {
	backoff := float64(c.config.InitialBackoff) * math.Pow(c.config.BackoffFactor, float64(attempt-1))
	if backoff > float64(c.config.MaxBackoff) {
		backoff = float64(c.config.MaxBackoff)
	}
	return time.Duration(backoff)
}

// isRetryable returns true if the status code indicates a retryable error
func (c *Client) isRetryable(statusCode int) bool {
	// Retry on rate limits and server errors
	return statusCode == http.StatusTooManyRequests ||
		statusCode == http.StatusServiceUnavailable ||
		statusCode == http.StatusBadGateway ||
		statusCode == http.StatusGatewayTimeout
}

// circuitBreaker implements a simple circuit breaker pattern
type circuitBreaker struct {
	mu               sync.RWMutex
	state            circuitState
	failures         int
	successes        int
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	lastFailure      time.Time
}

type circuitState int

const (
	circuitClosed circuitState = iota
	circuitOpen
	circuitHalfOpen
)

func newCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration) *circuitBreaker {
	return &circuitBreaker{
		state:            circuitClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
	}
}

// Allow checks if a request should be allowed through the circuit breaker
func (cb *circuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case circuitClosed:
		return true
	case circuitOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.state = circuitHalfOpen
			cb.successes = 0
			return true
		}
		return false
	case circuitHalfOpen:
		return true
	}
	return true
}

// RecordSuccess records a successful request
func (cb *circuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case circuitHalfOpen:
		cb.successes++
		if cb.successes >= cb.successThreshold {
			cb.state = circuitClosed
			cb.failures = 0
		}
	case circuitClosed:
		cb.failures = 0
	}
}

// RecordFailure records a failed request
func (cb *circuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	switch cb.state {
	case circuitClosed:
		if cb.failures >= cb.failureThreshold {
			cb.state = circuitOpen
		}
	case circuitHalfOpen:
		cb.state = circuitOpen
		cb.successes = 0
	}
}

// State returns the current circuit state (for testing/monitoring)
func (cb *circuitBreaker) State() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case circuitClosed:
		return "closed"
	case circuitOpen:
		return "open"
	case circuitHalfOpen:
		return "half-open"
	}
	return "unknown"
}
