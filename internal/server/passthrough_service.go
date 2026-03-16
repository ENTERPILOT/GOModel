package server

import (
	"strings"

	"github.com/labstack/echo/v5"

	"gomodel/internal/auditlog"
	"gomodel/internal/core"
)

type passthroughService struct {
	provider                     core.RoutableProvider
	logger                       auditlog.LoggerInterface
	normalizePassthroughV1Prefix bool
	enabledPassthroughProviders  map[string]struct{}
}

func (s *passthroughService) ProviderPassthrough(c *echo.Context) error {
	passthroughProvider, ok := s.provider.(core.RoutablePassthrough)
	if !ok {
		return handleError(c, core.NewInvalidRequestError("provider passthrough is not supported by the current provider router", nil))
	}

	providerType, endpoint, info, err := passthroughExecutionTarget(c, s.normalizePassthroughV1Prefix)
	if err != nil {
		return handleError(c, err)
	}
	if !isEnabledPassthroughProvider(providerType, s.enabledPassthroughProviders) {
		return handleError(c, s.unsupportedPassthroughProviderError(providerType))
	}

	ctx := c.Request().Context()
	requestID := strings.TrimSpace(c.Request().Header.Get("X-Request-ID"))
	if requestID == "" {
		requestID = strings.TrimSpace(core.GetRequestID(ctx))
	}
	if requestID != "" {
		ctx = core.WithRequestID(ctx, requestID)
	}
	resp, err := passthroughProvider.Passthrough(ctx, providerType, &core.PassthroughRequest{
		Method:   c.Request().Method,
		Endpoint: endpoint,
		Body:     c.Request().Body,
		Headers:  buildPassthroughHeaders(ctx, c.Request().Header),
	})
	if err != nil {
		return handleError(c, err)
	}

	auditlog.EnrichEntry(c, "passthrough", providerType)
	return s.proxyPassthroughResponse(c, providerType, endpoint, info, resp)
}
