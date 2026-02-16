package admin

// OverviewResponse is the JSON response for GET /admin/api/v1/overview.
type OverviewResponse struct {
	ModelCount    int    `json:"model_count"`
	ProviderCount int    `json:"provider_count"`
	Uptime        string `json:"uptime"`
	Version       string `json:"version"`
	GoVersion     string `json:"go_version"`
}

// AdminModelEntry represents a single model in the admin models list.
type AdminModelEntry struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	OwnedBy  string `json:"owned_by"`
}

// ModelsResponse is the JSON response for GET /admin/api/v1/models.
type ModelsResponse struct {
	Models []AdminModelEntry `json:"models"`
	Total  int               `json:"total"`
}
