package admin

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Models handles GET /admin/api/v1/models.
func (h *AdminHandler) Models(c echo.Context) error {
	coreModels := h.registry.ListModels()

	entries := make([]AdminModelEntry, 0, len(coreModels))
	for _, m := range coreModels {
		entries = append(entries, AdminModelEntry{
			ID:       m.ID,
			Provider: h.registry.GetProviderType(m.ID),
			OwnedBy:  m.OwnedBy,
		})
	}

	return c.JSON(http.StatusOK, ModelsResponse{
		Models: entries,
		Total:  len(entries),
	})
}
