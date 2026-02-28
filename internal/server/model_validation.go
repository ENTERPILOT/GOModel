package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/labstack/echo/v4"

	"gomodel/internal/auditlog"
	"gomodel/internal/core"
)

type contextKey string

const providerTypeKey contextKey = "providerType"

// ModelValidation validates model-interaction requests, enriches audit metadata,
// and propagates request-scoped values needed by downstream handlers.
func ModelValidation(provider core.RoutableProvider) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !auditlog.IsModelInteractionPath(c.Request().URL.Path) {
				return next(c)
			}
			if strings.HasPrefix(c.Request().URL.Path, "/v1/batches") {
				requestID := c.Request().Header.Get("X-Request-ID")
				ctx := core.WithRequestID(c.Request().Context(), requestID)
				c.SetRequest(c.Request().WithContext(ctx))
				return next(c)
			}

			bodyBytes, err := io.ReadAll(c.Request().Body)
			if err != nil {
				return handleError(c, core.NewInvalidRequestError("failed to read request body", err))
			}
			c.Request().Body = io.NopCloser(bytes.NewReader(bodyBytes))

			var peek struct {
				Model string `json:"model"`
			}
			if err := json.Unmarshal(bodyBytes, &peek); err != nil {
				return next(c)
			}

			if peek.Model == "" {
				return handleError(c, core.NewInvalidRequestError("model is required", nil))
			}

			if !provider.Supports(peek.Model) {
				return handleError(c, core.NewInvalidRequestError("unsupported model: "+peek.Model, nil))
			}

			providerType := provider.GetProviderType(peek.Model)
			c.Set(string(providerTypeKey), providerType)
			auditlog.EnrichEntry(c, peek.Model, providerType)

			requestID := c.Request().Header.Get("X-Request-ID")
			ctx := core.WithRequestID(c.Request().Context(), requestID)
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}

// GetProviderType returns the provider type set by ModelValidation for this request.
func GetProviderType(c echo.Context) string {
	if v, ok := c.Get(string(providerTypeKey)).(string); ok {
		return v
	}
	return ""
}

// ModelCtx returns the request context and resolved provider type.
func ModelCtx(c echo.Context) (context.Context, string) {
	return c.Request().Context(), GetProviderType(c)
}
