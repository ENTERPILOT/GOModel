package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		masterKey      string
		authHeader     string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "no master key configured - allows request",
			masterKey:      "",
			authHeader:     "",
			expectedStatus: http.StatusOK,
			expectedBody:   "ok",
		},
		{
			name:           "valid master key - allows request",
			masterKey:      "secret-key-123",
			authHeader:     "Bearer secret-key-123",
			expectedStatus: http.StatusOK,
			expectedBody:   "ok",
		},
		{
			name:           "missing authorization header - denies request",
			masterKey:      "secret-key-123",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":{"message":"missing authorization header","type":"authentication_error"}}`,
		},
		{
			name:           "invalid authorization format - denies request",
			masterKey:      "secret-key-123",
			authHeader:     "secret-key-123",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":{"message":"invalid authorization header format, expected 'Bearer \u003ctoken\u003e'","type":"authentication_error"}}`,
		},
		{
			name:           "invalid master key - denies request",
			masterKey:      "secret-key-123",
			authHeader:     "Bearer wrong-key",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":{"message":"invalid master key","type":"authentication_error"}}`,
		},
		{
			name:           "bearer prefix case sensitive - allows request",
			masterKey:      "secret-key-123",
			authHeader:     "Bearer secret-key-123",
			expectedStatus: http.StatusOK,
			expectedBody:   "ok",
		},
		{
			name:           "empty bearer token - denies request",
			masterKey:      "secret-key-123",
			authHeader:     "Bearer ",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":{"message":"invalid master key","type":"authentication_error"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()

			// Create a test handler that returns OK
			testHandler := func(c echo.Context) error {
				return c.String(http.StatusOK, "ok")
			}

			// Wrap the handler with auth middleware
			handler := AuthMiddleware(tt.masterKey)(testHandler)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Execute
			err := handler(c)

			// Assert
			if tt.expectedStatus == http.StatusOK {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedStatus, rec.Code)
				assert.Equal(t, tt.expectedBody, rec.Body.String())
			} else {
				// For error responses, the middleware returns the JSON directly
				require.NoError(t, err)
				assert.Equal(t, tt.expectedStatus, rec.Code)
				assert.JSONEq(t, tt.expectedBody, rec.Body.String())
			}
		})
	}
}

func TestAuthMiddleware_Integration(t *testing.T) {
	t.Run("with master key - protects all routes", func(t *testing.T) {
		e := echo.New()
		e.Use(AuthMiddleware("my-secret-key"))

		e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "success")
		})

		// Request without auth should fail
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		// Request with valid auth should succeed
		req = httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer my-secret-key")
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "success", rec.Body.String())
	})

	t.Run("without master key - allows all routes", func(t *testing.T) {
		e := echo.New()
		e.Use(AuthMiddleware(""))

		e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "success")
		})

		// Request without auth should succeed
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "success", rec.Body.String())
	})
}
