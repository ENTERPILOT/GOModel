// Package admin provides HTTP handlers for the admin API.
package admin

import (
	"time"

	"gomodel/internal/core"
)

// RegistryInfo provides read-only access to model registry data.
// *providers.ModelRegistry satisfies this interface.
type RegistryInfo interface {
	ModelCount() int
	ProviderCount() int
	ListModels() []core.Model
	GetProviderType(model string) string
}

// AdminHandler serves admin API endpoints.
type AdminHandler struct {
	registry  RegistryInfo
	startTime time.Time
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(registry RegistryInfo) *AdminHandler {
	return &AdminHandler{
		registry:  registry,
		startTime: time.Now(),
	}
}
