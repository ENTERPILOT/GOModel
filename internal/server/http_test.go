package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		config         *Config
		requestPath    string
		expectedStatus int
		expectBody     string // substring to check in response body
	}{
		{
			name: "metrics enabled - default endpoint accessible",
			config: &Config{
				MetricsEnabled:  true,
				MetricsEndpoint: "/metrics",
			},
			requestPath:    "/metrics",
			expectedStatus: http.StatusOK,
			expectBody:     "go_goroutines", // Standard Go runtime metric
		},
		{
			name: "metrics enabled - empty endpoint defaults to /metrics",
			config: &Config{
				MetricsEnabled:  true,
				MetricsEndpoint: "",
			},
			requestPath:    "/metrics",
			expectedStatus: http.StatusOK,
			expectBody:     "go_goroutines",
		},
		{
			name: "metrics disabled - endpoint returns 404",
			config: &Config{
				MetricsEnabled:  false,
				MetricsEndpoint: "/metrics",
			},
			requestPath:    "/metrics",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "nil config - metrics disabled by default",
			config:         nil,
			requestPath:    "/metrics",
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "custom metrics endpoint path",
			config: &Config{
				MetricsEnabled:  true,
				MetricsEndpoint: "/custom-metrics",
			},
			requestPath:    "/custom-metrics",
			expectedStatus: http.StatusOK,
			expectBody:     "go_goroutines",
		},
		{
			name: "custom endpoint - default path returns 404",
			config: &Config{
				MetricsEnabled:  true,
				MetricsEndpoint: "/custom-metrics",
			},
			requestPath:    "/metrics",
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "metrics endpoint with nested path",
			config: &Config{
				MetricsEnabled:  true,
				MetricsEndpoint: "/api/v1/metrics",
			},
			requestPath:    "/api/v1/metrics",
			expectedStatus: http.StatusOK,
			expectBody:     "go_goroutines",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockProvider{}
			srv := New(mock, tt.config)

			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			if tt.expectBody != "" && !strings.Contains(rec.Body.String(), tt.expectBody) {
				t.Errorf("expected body to contain %q, got: %s", tt.expectBody, rec.Body.String())
			}
		})
	}
}

func TestMetricsEndpointReturnsPrometheusFormat(t *testing.T) {
	mock := &mockProvider{}
	srv := New(mock, &Config{
		MetricsEnabled:  true,
		MetricsEndpoint: "/metrics",
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	// Check for Prometheus text format indicators
	// Prometheus metrics should contain HELP and TYPE comments
	if !strings.Contains(body, "# HELP") {
		t.Error("response should contain Prometheus HELP comments")
	}
	if !strings.Contains(body, "# TYPE") {
		t.Error("response should contain Prometheus TYPE comments")
	}

	// Check for standard Go runtime metrics that are always present
	standardMetrics := []string{
		"go_goroutines",
		"go_gc_duration_seconds",
		"go_memstats_alloc_bytes",
		"process_cpu_seconds_total",
	}

	for _, metric := range standardMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("response should contain standard metric %q", metric)
		}
	}

	// Check Content-Type header
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("expected Content-Type to contain text/plain, got %s", contentType)
	}
}

func TestServerWithMasterKeyAndMetrics(t *testing.T) {
	mock := &mockProvider{}
	srv := New(mock, &Config{
		MasterKey:       "test-secret-key",
		MetricsEnabled:  true,
		MetricsEndpoint: "/metrics",
	})

	t.Run("metrics endpoint is public even when master key is set", func(t *testing.T) {
		// Metrics endpoint should be accessible without auth for Prometheus scraping
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		// Should return 200 - metrics is public for load balancers and monitoring
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 for public metrics endpoint, got %d", rec.Code)
		}
	})

	t.Run("health endpoint is public even when master key is set", func(t *testing.T) {
		// Health endpoint should be accessible without auth for load balancer health checks
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		// Should return 200 - health is public for load balancers
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 for public health endpoint, got %d", rec.Code)
		}
	})

	t.Run("API endpoints require auth when master key is set", func(t *testing.T) {
		// API endpoints should require auth
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		// Should return 401 - API requires auth
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401 for protected API endpoint, got %d", rec.Code)
		}
	})

	t.Run("API endpoints accessible with valid auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		req.Header.Set("Authorization", "Bearer test-secret-key")
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		// Should return 200 with valid auth
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 with valid auth, got %d", rec.Code)
		}
	})
}

func TestHealthEndpointAlwaysAvailable(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name:   "nil config",
			config: nil,
		},
		{
			name: "metrics disabled",
			config: &Config{
				MetricsEnabled: false,
			},
		},
		{
			name: "metrics enabled",
			config: &Config{
				MetricsEnabled:  true,
				MetricsEndpoint: "/metrics",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockProvider{}
			srv := New(mock, tt.config)

			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", rec.Code)
			}
		})
	}
}
