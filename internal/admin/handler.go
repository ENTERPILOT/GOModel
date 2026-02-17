// Package admin provides the admin REST API and dashboard for GOModel.
package admin

import (
	"net/http"
	"strconv"
	"time"

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

var validIntervals = map[string]bool{
	"daily":   true,
	"weekly":  true,
	"monthly": true,
	"yearly":  true,
}

// parseUsageParams extracts UsageQueryParams from the request query string.
func parseUsageParams(c echo.Context) usage.UsageQueryParams {
	var params usage.UsageQueryParams

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	startStr := c.QueryParam("start_date")
	endStr := c.QueryParam("end_date")

	var startParsed, endParsed bool

	if startStr != "" {
		if t, err := time.Parse("2006-01-02", startStr); err == nil {
			params.StartDate = t
			startParsed = true
		}
	}

	if endStr != "" {
		if t, err := time.Parse("2006-01-02", endStr); err == nil {
			params.EndDate = t
			endParsed = true
		}
	}

	if startParsed || endParsed {
		// Fill in missing side
		if !startParsed {
			params.StartDate = params.EndDate.AddDate(0, 0, -29)
		}
		if !endParsed {
			params.EndDate = today
		}
	} else {
		// Fall back to days param
		days := 30
		if d := c.QueryParam("days"); d != "" {
			if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
				days = parsed
			}
		}
		params.EndDate = today
		params.StartDate = today.AddDate(0, 0, -(days - 1))
	}

	// Parse interval
	params.Interval = c.QueryParam("interval")
	if !validIntervals[params.Interval] {
		params.Interval = "daily"
	}

	return params
}

// UsageSummary handles GET /admin/api/v1/usage/summary
func (h *Handler) UsageSummary(c echo.Context) error {
	if h.usageReader == nil {
		return c.JSON(http.StatusOK, usage.UsageSummary{})
	}

	params := parseUsageParams(c)

	summary, err := h.usageReader.GetSummary(c.Request().Context(), params)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to fetch usage summary",
		})
	}

	return c.JSON(http.StatusOK, summary)
}

// DailyUsage handles GET /admin/api/v1/usage/daily
func (h *Handler) DailyUsage(c echo.Context) error {
	if h.usageReader == nil {
		return c.JSON(http.StatusOK, []usage.DailyUsage{})
	}

	params := parseUsageParams(c)

	daily, err := h.usageReader.GetDailyUsage(c.Request().Context(), params)
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
