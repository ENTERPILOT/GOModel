package server

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// AuthMiddleware creates an Echo middleware that validates the master key
// if it's configured. If masterKey is empty, no authentication is required.
func AuthMiddleware(masterKey string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// If no master key is configured, allow all requests
			if masterKey == "" {
				return next(c)
			}

			// Get Authorization header
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": map[string]interface{}{
						"type":    "authentication_error",
						"message": "missing authorization header",
					},
				})
			}

			// Extract Bearer token
			const prefix = "Bearer "
			if !strings.HasPrefix(authHeader, prefix) {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": map[string]interface{}{
						"type":    "authentication_error",
						"message": "invalid authorization header format, expected 'Bearer <token>'",
					},
				})
			}

			token := strings.TrimPrefix(authHeader, prefix)
			if token != masterKey {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": map[string]interface{}{
						"type":    "authentication_error",
						"message": "invalid master key",
					},
				})
			}

			// Authentication successful, proceed to next handler
			return next(c)
		}
	}
}
