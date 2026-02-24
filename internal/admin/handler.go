// Package admin provides the admin REST API and dashboard for GOModel.
package admin

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"gomodel/internal/core"
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
// Returns an error if date parameters are provided but malformed.
func parseUsageParams(c echo.Context) (usage.UsageQueryParams, error) {
	var params usage.UsageQueryParams

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	startStr := c.QueryParam("start_date")
	endStr := c.QueryParam("end_date")

	var startParsed, endParsed bool

	if startStr != "" {
		t, err := time.Parse("2006-01-02", startStr)
		if err != nil {
			return params, core.NewInvalidRequestError("invalid start_date format, expected YYYY-MM-DD", nil)
		}
		params.StartDate = t
		startParsed = true
	}

	if endStr != "" {
		t, err := time.Parse("2006-01-02", endStr)
		if err != nil {
			return params, core.NewInvalidRequestError("invalid end_date format, expected YYYY-MM-DD", nil)
		}
		params.EndDate = t
		endParsed = true
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

	return params, nil
}

// handleError converts errors to appropriate HTTP responses, matching the
// format used by the main API handlers in the server package.
func handleError(c echo.Context, err error) error {
	var gatewayErr *core.GatewayError
	if errors.As(err, &gatewayErr) {
		return c.JSON(gatewayErr.HTTPStatusCode(), gatewayErr.ToJSON())
	}

	return c.JSON(http.StatusInternalServerError, map[string]interface{}{
		"error": map[string]interface{}{
			"type":    "internal_error",
			"message": "an unexpected error occurred",
		},
	})
}

// UsageSummary handles GET /admin/api/v1/usage/summary
//
// @Summary      Get usage summary
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        days        query     int     false  "Number of days (default 30)"
// @Param        start_date  query     string  false  "Start date (YYYY-MM-DD)"
// @Param        end_date    query     string  false  "End date (YYYY-MM-DD)"
// @Success      200  {object}  usage.UsageSummary
// @Failure      400  {object}  core.GatewayError
// @Failure      401  {object}  core.GatewayError
// @Router       /admin/api/v1/usage/summary [get]
func (h *Handler) UsageSummary(c echo.Context) error {
	if h.usageReader == nil {
		return c.JSON(http.StatusOK, usage.UsageSummary{})
	}

	params, err := parseUsageParams(c)
	if err != nil {
		return handleError(c, err)
	}

	summary, err := h.usageReader.GetSummary(c.Request().Context(), params)
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(http.StatusOK, summary)
}

// DailyUsage handles GET /admin/api/v1/usage/daily
//
// @Summary      Get usage breakdown by period
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        days        query     int     false  "Number of days (default 30)"
// @Param        start_date  query     string  false  "Start date (YYYY-MM-DD)"
// @Param        end_date    query     string  false  "End date (YYYY-MM-DD)"
// @Param        interval    query     string  false  "Grouping interval: daily, weekly, monthly, yearly (default daily)"
// @Success      200  {array}   usage.DailyUsage
// @Failure      400  {object}  core.GatewayError
// @Failure      401  {object}  core.GatewayError
// @Router       /admin/api/v1/usage/daily [get]
func (h *Handler) DailyUsage(c echo.Context) error {
	if h.usageReader == nil {
		return c.JSON(http.StatusOK, []usage.DailyUsage{})
	}

	params, err := parseUsageParams(c)
	if err != nil {
		return handleError(c, err)
	}

	daily, err := h.usageReader.GetDailyUsage(c.Request().Context(), params)
	if err != nil {
		return handleError(c, err)
	}

	if daily == nil {
		daily = []usage.DailyUsage{}
	}

	return c.JSON(http.StatusOK, daily)
}

// UsageByModel handles GET /admin/api/v1/usage/models
func (h *Handler) UsageByModel(c echo.Context) error {
	if h.usageReader == nil {
		return c.JSON(http.StatusOK, []usage.ModelUsage{})
	}

	params, err := parseUsageParams(c)
	if err != nil {
		return handleError(c, err)
	}

	models, err := h.usageReader.GetUsageByModel(c.Request().Context(), params)
	if err != nil {
		return handleError(c, err)
	}

	if models == nil {
		models = []usage.ModelUsage{}
	}

	return c.JSON(http.StatusOK, models)
}

// UsageLog handles GET /admin/api/v1/usage/log
func (h *Handler) UsageLog(c echo.Context) error {
	if h.usageReader == nil {
		return c.JSON(http.StatusOK, usage.UsageLogResult{
			Entries: []usage.UsageLogEntry{},
		})
	}

	baseParams, err := parseUsageParams(c)
	if err != nil {
		return handleError(c, err)
	}

	params := usage.UsageLogParams{
		UsageQueryParams: baseParams,
		Model:            c.QueryParam("model"),
		Provider:         c.QueryParam("provider"),
		Search:           c.QueryParam("search"),
	}

	if l := c.QueryParam("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			params.Limit = parsed
		}
	}
	if o := c.QueryParam("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			params.Offset = parsed
		}
	}

	result, err := h.usageReader.GetUsageLog(c.Request().Context(), params)
	if err != nil {
		return handleError(c, err)
	}

	if result.Entries == nil {
		result.Entries = []usage.UsageLogEntry{}
	}

	return c.JSON(http.StatusOK, result)
}

// ListModels handles GET /admin/api/v1/models
<<<<<<< feature/pricing-estimation
// Supports optional ?category= query param for filtering by model category.
=======
//
// @Summary      List all registered models with provider info
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}  providers.ModelWithProvider
// @Failure      401  {object}  core.GatewayError
// @Router       /admin/api/v1/models [get]
>>>>>>> main
func (h *Handler) ListModels(c echo.Context) error {
	if h.registry == nil {
		return c.JSON(http.StatusOK, []providers.ModelWithProvider{})
	}

	var models []providers.ModelWithProvider
	if cat := core.ModelCategory(c.QueryParam("category")); cat != "" && cat != core.CategoryAll {
		models = h.registry.ListModelsWithProviderByCategory(cat)
	} else {
		models = h.registry.ListModelsWithProvider()
	}

	if models == nil {
		models = []providers.ModelWithProvider{}
	}

	return c.JSON(http.StatusOK, models)
}

// ListCategories handles GET /admin/api/v1/models/categories
func (h *Handler) ListCategories(c echo.Context) error {
	if h.registry == nil {
		return c.JSON(http.StatusOK, []providers.CategoryCount{})
	}

	return c.JSON(http.StatusOK, h.registry.GetCategoryCounts())
}
