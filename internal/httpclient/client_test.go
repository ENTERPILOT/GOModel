package httpclient

import (
	"net/http"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxIdleConns != 100 {
		t.Errorf("Expected MaxIdleConns to be 100, got %d", config.MaxIdleConns)
	}

	if config.MaxIdleConnsPerHost != 100 {
		t.Errorf("Expected MaxIdleConnsPerHost to be 100, got %d", config.MaxIdleConnsPerHost)
	}

	if config.IdleConnTimeout != 90*time.Second {
		t.Errorf("Expected IdleConnTimeout to be 90s, got %v", config.IdleConnTimeout)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected Timeout to be 30s, got %v", config.Timeout)
	}

	if config.DialTimeout != 30*time.Second {
		t.Errorf("Expected DialTimeout to be 30s, got %v", config.DialTimeout)
	}

	if config.KeepAlive != 30*time.Second {
		t.Errorf("Expected KeepAlive to be 30s, got %v", config.KeepAlive)
	}

	if config.TLSHandshakeTimeout != 10*time.Second {
		t.Errorf("Expected TLSHandshakeTimeout to be 10s, got %v", config.TLSHandshakeTimeout)
	}

	if config.ResponseHeaderTimeout != 10*time.Second {
		t.Errorf("Expected ResponseHeaderTimeout to be 10s, got %v", config.ResponseHeaderTimeout)
	}
}

func TestNewHTTPClient(t *testing.T) {
	tests := []struct {
		name   string
		config *ClientConfig
	}{
		{
			name:   "nil config uses defaults",
			config: nil,
		},
		{
			name: "custom config",
			config: &ClientConfig{
				MaxIdleConns:          50,
				MaxIdleConnsPerHost:   25,
				IdleConnTimeout:       60 * time.Second,
				Timeout:               15 * time.Second,
				DialTimeout:           10 * time.Second,
				KeepAlive:             15 * time.Second,
				TLSHandshakeTimeout:   5 * time.Second,
				ResponseHeaderTimeout: 5 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewHTTPClient(tt.config)

			if client == nil {
				t.Fatal("Expected client to be non-nil")
			}

			if client.Transport == nil {
				t.Fatal("Expected transport to be non-nil")
			}

			transport, ok := client.Transport.(*http.Transport)
			if !ok {
				t.Fatal("Expected transport to be *http.Transport")
			}

			expectedConfig := tt.config
			if expectedConfig == nil {
				cfg := DefaultConfig()
				expectedConfig = &cfg
			}

			// Verify transport settings
			if transport.MaxIdleConns != expectedConfig.MaxIdleConns {
				t.Errorf("Expected MaxIdleConns to be %d, got %d", expectedConfig.MaxIdleConns, transport.MaxIdleConns)
			}

			if transport.MaxIdleConnsPerHost != expectedConfig.MaxIdleConnsPerHost {
				t.Errorf("Expected MaxIdleConnsPerHost to be %d, got %d", expectedConfig.MaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
			}

			if transport.IdleConnTimeout != expectedConfig.IdleConnTimeout {
				t.Errorf("Expected IdleConnTimeout to be %v, got %v", expectedConfig.IdleConnTimeout, transport.IdleConnTimeout)
			}

			if client.Timeout != expectedConfig.Timeout {
				t.Errorf("Expected client Timeout to be %v, got %v", expectedConfig.Timeout, client.Timeout)
			}

			if transport.TLSHandshakeTimeout != expectedConfig.TLSHandshakeTimeout {
				t.Errorf("Expected TLSHandshakeTimeout to be %v, got %v", expectedConfig.TLSHandshakeTimeout, transport.TLSHandshakeTimeout)
			}

			if transport.ResponseHeaderTimeout != expectedConfig.ResponseHeaderTimeout {
				t.Errorf("Expected ResponseHeaderTimeout to be %v, got %v", expectedConfig.ResponseHeaderTimeout, transport.ResponseHeaderTimeout)
			}

			// Verify ForceAttemptHTTP2 is enabled
			if !transport.ForceAttemptHTTP2 {
				t.Error("Expected ForceAttemptHTTP2 to be enabled")
			}

			// Verify Proxy is set
			if transport.Proxy == nil {
				t.Error("Expected Proxy to be set")
			}
		})
	}
}

func TestNewDefaultHTTPClient(t *testing.T) {
	client := NewDefaultHTTPClient()

	if client == nil {
		t.Fatal("Expected client to be non-nil")
	}

	if client.Transport == nil {
		t.Fatal("Expected transport to be non-nil")
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected transport to be *http.Transport")
	}

	defaultConfig := DefaultConfig()

	// Verify it uses default configuration
	if transport.MaxIdleConns != defaultConfig.MaxIdleConns {
		t.Errorf("Expected MaxIdleConns to be %d, got %d", defaultConfig.MaxIdleConns, transport.MaxIdleConns)
	}

	if transport.MaxIdleConnsPerHost != defaultConfig.MaxIdleConnsPerHost {
		t.Errorf("Expected MaxIdleConnsPerHost to be %d, got %d", defaultConfig.MaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
	}

	if client.Timeout != defaultConfig.Timeout {
		t.Errorf("Expected client Timeout to be %v, got %v", defaultConfig.Timeout, client.Timeout)
	}
}

func TestHTTPClientIsReusable(t *testing.T) {
	// Test that multiple calls return different client instances (not a singleton)
	// but with the same configuration
	client1 := NewDefaultHTTPClient()
	client2 := NewDefaultHTTPClient()

	if client1 == client2 {
		t.Error("Expected different client instances")
	}

	// But they should have the same configuration
	transport1 := client1.Transport.(*http.Transport)
	transport2 := client2.Transport.(*http.Transport)

	if transport1.MaxIdleConns != transport2.MaxIdleConns {
		t.Error("Expected same MaxIdleConns configuration")
	}

	if client1.Timeout != client2.Timeout {
		t.Error("Expected same Timeout configuration")
	}
}

func TestClientConfigZeroValues(t *testing.T) {
	// Test that zero values in config are still applied (not replaced with defaults)
	config := &ClientConfig{
		MaxIdleConns:          0,
		MaxIdleConnsPerHost:   0,
		IdleConnTimeout:       0,
		Timeout:               0,
		DialTimeout:           0,
		KeepAlive:             0,
		TLSHandshakeTimeout:   0,
		ResponseHeaderTimeout: 0,
	}

	client := NewHTTPClient(config)
	transport := client.Transport.(*http.Transport)

	// Zero values should be preserved (not replaced with defaults)
	if transport.MaxIdleConns != 0 {
		t.Errorf("Expected MaxIdleConns to be 0, got %d", transport.MaxIdleConns)
	}

	if client.Timeout != 0 {
		t.Errorf("Expected Timeout to be 0, got %v", client.Timeout)
	}
}
