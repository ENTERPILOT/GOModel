package admin

import (
	"net/http"
	"runtime"
	"time"

	"github.com/labstack/echo/v4"

	"gomodel/internal/version"
)

// Overview handles GET /admin/api/v1/overview.
func (h *AdminHandler) Overview(c echo.Context) error {
	uptime := time.Since(h.startTime).Round(time.Second)

	return c.JSON(http.StatusOK, OverviewResponse{
		ModelCount:    h.registry.ModelCount(),
		ProviderCount: h.registry.ProviderCount(),
		Uptime:        uptime.String(),
		Version:       version.Version,
		GoVersion:     runtime.Version(),
	})
}
