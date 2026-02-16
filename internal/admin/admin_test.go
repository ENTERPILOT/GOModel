package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gomodel/internal/core"
)

// mockRegistry implements RegistryInfo for testing.
type mockRegistry struct {
	models    []core.Model
	providers map[string]string // model ID -> provider type
}

func (m *mockRegistry) ModelCount() int { return len(m.models) }
func (m *mockRegistry) ProviderCount() int {
	seen := map[string]bool{}
	for _, pt := range m.providers {
		seen[pt] = true
	}
	return len(seen)
}
func (m *mockRegistry) ListModels() []core.Model { return m.models }
func (m *mockRegistry) GetProviderType(model string) string {
	return m.providers[model]
}

func newTestHandler() *AdminHandler {
	reg := &mockRegistry{
		models: []core.Model{
			{ID: "gpt-4", OwnedBy: "openai"},
			{ID: "claude-3-opus", OwnedBy: "anthropic"},
			{ID: "gemini-pro", OwnedBy: "google"},
		},
		providers: map[string]string{
			"gpt-4":        "openai",
			"claude-3-opus": "anthropic",
			"gemini-pro":   "gemini",
		},
	}
	return NewAdminHandler(reg)
}

func TestOverview(t *testing.T) {
	h := newTestHandler()
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/admin/api/v1/overview", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Overview(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp OverviewResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 3, resp.ModelCount)
	assert.Equal(t, 3, resp.ProviderCount)
	assert.NotEmpty(t, resp.Uptime)
	assert.NotEmpty(t, resp.Version)
	assert.NotEmpty(t, resp.GoVersion)
}

func TestModels(t *testing.T) {
	h := newTestHandler()
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/admin/api/v1/models", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Models(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ModelsResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 3, resp.Total)
	assert.Len(t, resp.Models, 3)

	// Verify model entries have provider types
	providersByModel := map[string]string{}
	for _, m := range resp.Models {
		providersByModel[m.ID] = m.Provider
	}
	assert.Equal(t, "openai", providersByModel["gpt-4"])
	assert.Equal(t, "anthropic", providersByModel["claude-3-opus"])
	assert.Equal(t, "gemini", providersByModel["gemini-pro"])
}

func TestModelsEmpty(t *testing.T) {
	reg := &mockRegistry{
		models:    []core.Model{},
		providers: map[string]string{},
	}
	h := NewAdminHandler(reg)
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/admin/api/v1/models", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Models(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ModelsResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 0, resp.Total)
	assert.Empty(t, resp.Models)
}
