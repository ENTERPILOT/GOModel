package server

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"gomodel/internal/auditlog"
	"gomodel/internal/core"
)

// handleError converts gateway errors to appropriate HTTP responses.
func handleError(c *echo.Context, err error) error {
	var gatewayErr *core.GatewayError
	if errors.As(err, &gatewayErr) {
		auditlog.EnrichEntryWithError(c, string(gatewayErr.Type), gatewayErr.Message)
		return c.JSON(gatewayErr.HTTPStatusCode(), gatewayErr.ToJSON())
	}

	gatewayErr = core.NewProviderError("", http.StatusInternalServerError, "an unexpected error occurred", err)
	auditlog.EnrichEntryWithError(c, string(gatewayErr.Type), gatewayErr.Message)
	return c.JSON(gatewayErr.HTTPStatusCode(), gatewayErr.ToJSON())
}
