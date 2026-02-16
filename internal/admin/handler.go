// Package admin provides the admin REST API and dashboard for GOModel.
package admin

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"gomodel/internal/providers"
	"gomodel/internal/usage"
)

// Handler serves admin API endpoints.
type Handler struct {
	usageReader usage.UsageReader
	registry    *providers.ModelRegistry
}

// NewHandler creates a new admin API handler.
// usageReader may be nil if usage tracking is not available.
func NewHandler(reader usage.UsageReader, registry *providers.ModelRegistry) *Handler {
	return &Handler{
		usageReader: reader,
		registry:    registry,
	}
}

// UsageSummary handles GET /admin/api/v1/usage/summary?days=30
func (h *Handler) UsageSummary(c echo.Context) error {
	if h.usageReader == nil {
		return c.JSON(http.StatusOK, usage.UsageSummary{})
	}

	days := 30
	if d := c.QueryParam("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}

	summary, err := h.usageReader.GetSummary(c.Request().Context(), days)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to fetch usage summary",
		})
	}

	return c.JSON(http.StatusOK, summary)
}

// DailyUsage handles GET /admin/api/v1/usage/daily?days=30
func (h *Handler) DailyUsage(c echo.Context) error {
	if h.usageReader == nil {
		return c.JSON(http.StatusOK, []usage.DailyUsage{})
	}

	days := 30
	if d := c.QueryParam("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}

	daily, err := h.usageReader.GetDailyUsage(c.Request().Context(), days)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to fetch daily usage",
		})
	}

	if daily == nil {
		daily = []usage.DailyUsage{}
	}

	return c.JSON(http.StatusOK, daily)
}

// ListModels handles GET /admin/api/v1/models
func (h *Handler) ListModels(c echo.Context) error {
	if h.registry == nil {
		return c.JSON(http.StatusOK, []providers.ModelWithProvider{})
	}

	models := h.registry.ListModelsWithProvider()
	if models == nil {
		models = []providers.ModelWithProvider{}
	}

	return c.JSON(http.StatusOK, models)
}
