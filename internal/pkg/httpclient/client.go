// Package httpclient provides a centralized HTTP client factory with unified configuration.
package httpclient

import (
	"net"
	"net/http"
	"time"
)

// ClientConfig holds configuration options for creating HTTP clients
type ClientConfig struct {
	// MaxIdleConns controls the maximum number of idle (keep-alive) connections across all hosts
	MaxIdleConns int

	// MaxIdleConnsPerHost controls the maximum idle (keep-alive) connections to keep per-host
	MaxIdleConnsPerHost int

	// IdleConnTimeout is the maximum amount of time an idle (keep-alive) connection will remain idle before closing itself
	IdleConnTimeout time.Duration

	// Timeout specifies a time limit for requests made by the client
	Timeout time.Duration

	// DialTimeout is the maximum amount of time a dial will wait for a connect to complete
	DialTimeout time.Duration

	// KeepAlive specifies the interval between keep-alive probes for an active network connection
	KeepAlive time.Duration

	// TLSHandshakeTimeout specifies the maximum amount of time to wait for a TLS handshake
	TLSHandshakeTimeout time.Duration

	// ResponseHeaderTimeout specifies the amount of time to wait for a server's response headers
	ResponseHeaderTimeout time.Duration
}

// DefaultConfig returns a ClientConfig with sensible defaults for API clients
func DefaultConfig() ClientConfig {
	return ClientConfig{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		Timeout:               30 * time.Second,
		DialTimeout:           30 * time.Second,
		KeepAlive:             30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}
}

// NewHTTPClient creates a new HTTP client with the provided configuration.
// If config is nil, DefaultConfig() is used.
func NewHTTPClient(config *ClientConfig) *http.Client {
	if config == nil {
		cfg := DefaultConfig()
		config = &cfg
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: config.KeepAlive,
		}).DialContext,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
		ForceAttemptHTTP2:     true,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}
}

// NewDefaultHTTPClient creates a new HTTP client with default configuration.
// This is a convenience function equivalent to NewHTTPClient(nil).
func NewDefaultHTTPClient() *http.Client {
	return NewHTTPClient(nil)
}
