package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMetricsEndpointPathCollision verifies that metrics endpoint paths under /v1/*
// are rejected and fall back to /metrics to prevent auth bypass
func TestMetricsEndpointPathCollision(t *testing.T) {
	mock := &mockProvider{}

	t.Run("metrics at /v1/metrics falls back to /metrics", func(t *testing.T) {
		srv := New(mock, &Config{
			MasterKey:       "secret-key",
			MetricsEnabled:  true,
			MetricsEndpoint: "/v1/metrics", // Should be rejected and fall back to /metrics
		})

		// /v1/metrics should require auth (not be the metrics endpoint)
		req := httptest.NewRequest(http.MethodGet, "/v1/metrics", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 for /v1/metrics (validation should reject this path), got %d", rec.Code)
		}

		// Metrics should be available at /metrics instead
		req2 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec2 := httptest.NewRecorder()
		srv.ServeHTTP(rec2, req2)

		if rec2.Code != http.StatusOK {
			t.Errorf("Expected 200 for /metrics (fallback path), got %d", rec2.Code)
		}
	})

	t.Run("metrics at /v1/chat/completions falls back to /metrics", func(t *testing.T) {
		srv := New(mock, &Config{
			MasterKey:       "secret-key",
			MetricsEnabled:  true,
			MetricsEndpoint: "/v1/chat/completions", // Should be rejected
		})

		// /v1/chat/completions should require auth
		req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 for /v1/chat/completions, got %d", rec.Code)
		}

		// Metrics should be at /metrics
		req2 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec2 := httptest.NewRecorder()
		srv.ServeHTTP(rec2, req2)

		if rec2.Code != http.StatusOK {
			t.Errorf("Expected 200 for /metrics, got %d", rec2.Code)
		}
	})

	t.Run("/v10/metrics is allowed (not under /v1/)", func(t *testing.T) {
		srv := New(mock, &Config{
			MasterKey:       "secret-key",
			MetricsEnabled:  true,
			MetricsEndpoint: "/v10/metrics", // Should be allowed - not under /v1/
		})

		// /v10/metrics should work as metrics endpoint
		req := httptest.NewRequest(http.MethodGet, "/v10/metrics", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected 200 for /v10/metrics (allowed path), got %d", rec.Code)
		}
	})
}

// TestBodyLimitHTTPMethodCoverage tests that body limits apply to all HTTP methods
func TestBodyLimitHTTPMethodCoverage(t *testing.T) {
	mock := &mockProvider{}
	srv := New(mock, &Config{
		MasterKey:      "",
		MetricsEnabled: false,
	})

	// Create a body larger than 10MB
	largeBody := strings.Repeat("x", 11*1024*1024)

	// Test GET with large body (unusual but possible)
	t.Run("GET with large body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", strings.NewReader(largeBody))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("GET request with 11MB body should be rejected (body limit is 10MB), got %d", rec.Code)
			t.Log("This could allow DoS via GET requests with large bodies")
		}
	})

	// Test POST with large body
	t.Run("POST with large body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(largeBody))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("POST request with 11MB body should be rejected, got %d", rec.Code)
		}
	})
}

// TestHealthEndpointNotAffectedByBodyLimit tests that health endpoint
// is not subject to API group body limits
func TestHealthEndpointNotAffectedByBodyLimit(t *testing.T) {
	mock := &mockProvider{}
	srv := New(mock, nil)

	// Health endpoint should work even with a body (though unusual)
	// This tests that it's truly outside the /v1 group
	largeBody := strings.Repeat("x", 11*1024*1024)
	req := httptest.NewRequest(http.MethodGet, "/health", strings.NewReader(largeBody))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	// Health should return 200, not 413, because it's outside the /v1 group
	if rec.Code != http.StatusOK {
		t.Errorf("Health endpoint should not have body limit, got status %d", rec.Code)
	}
}

// TestMetricsEndpointPathTraversal tests that path traversal cannot bypass validation
func TestMetricsEndpointPathTraversal(t *testing.T) {
	mock := &mockProvider{}

	t.Run("path traversal to /v1/ is blocked after normalization", func(t *testing.T) {
		// /foo/../v1/admin normalizes to /v1/admin which should be rejected
		srv := New(mock, &Config{
			MasterKey:       "secret",
			MetricsEnabled:  true,
			MetricsEndpoint: "/foo/../v1/admin",
		})

		// Metrics should fall back to /metrics since normalized path is under /v1/
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected metrics at /metrics after fallback, got %d", rec.Code)
		}

		// /v1/admin should require auth (not be the metrics endpoint)
		req2 := httptest.NewRequest(http.MethodGet, "/v1/admin", nil)
		rec2 := httptest.NewRecorder()
		srv.ServeHTTP(rec2, req2)

		if rec2.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 for /v1/admin, got %d", rec2.Code)
		}
	})

	t.Run("path traversing away from /v1 is allowed", func(t *testing.T) {
		// /v1/../admin normalizes to /admin which is NOT under /v1/
		srv := New(mock, &Config{
			MasterKey:       "secret",
			MetricsEnabled:  true,
			MetricsEndpoint: "/v1/../admin",
		})

		// After normalization, /v1/../admin -> /admin, which is allowed
		// So metrics should be served at /v1/../admin (which Echo normalizes to /admin)
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected 200 for /admin (normalized path is allowed), got %d", rec.Code)
		}
	})
}
