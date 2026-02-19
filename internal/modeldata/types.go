// Package modeldata provides fetching, parsing, and merging of the external
// AI model metadata registry (models.json) for enriching GoModel's model data.
package modeldata

// ModelList represents the top-level structure of models.json.
type ModelList struct {
	Version        int                          `json:"version"`
	UpdatedAt      string                       `json:"updated_at"`
	Providers      map[string]ProviderEntry     `json:"providers"`
	Models         map[string]ModelEntry        `json:"models"`
	ProviderModels map[string]ProviderModelEntry `json:"provider_models"`

	// providerModelByActualID maps "providerType/actualModelID" → composite key
	// in ProviderModels, enabling reverse lookup from a provider's response model
	// (e.g., "openai/gpt-4o-2024-08-06") to the canonical registry key
	// (e.g., "openai/gpt-4o"). Built by buildReverseIndex().
	providerModelByActualID map[string]string
}

// buildReverseIndex populates providerModelByActualID from ProviderModels entries
// that have a non-nil provider_model_id differing from the key's model portion.
func (l *ModelList) buildReverseIndex() {
	l.providerModelByActualID = make(map[string]string)
	for compositeKey, pm := range l.ProviderModels {
		if pm.ProviderModelID == nil {
			continue
		}
		// compositeKey is "providerType/modelID"
		// Extract provider type from the composite key
		slashIdx := -1
		for i, ch := range compositeKey {
			if ch == '/' {
				slashIdx = i
				break
			}
		}
		if slashIdx < 0 {
			continue
		}
		providerType := compositeKey[:slashIdx]
		actualID := *pm.ProviderModelID
		reverseKey := providerType + "/" + actualID
		// Only add if the actual ID differs from the key's model portion
		if reverseKey != compositeKey {
			l.providerModelByActualID[reverseKey] = compositeKey
		}
	}
}

// ProviderEntry represents a provider in the registry.
type ProviderEntry struct {
	DisplayName    string     `json:"display_name"`
	Website        *string    `json:"website"`
	DocsURL        *string    `json:"docs_url"`
	PricingURL     *string    `json:"pricing_url"`
	StatusURL      *string    `json:"status_url"`
	APIType        string     `json:"api_type"`
	DefaultBaseURL *string    `json:"default_base_url"`
	SupportedModes []string   `json:"supported_modes"`
	DefaultRateLimits *RateLimits `json:"default_rate_limits"`
}

// ModelEntry represents a model in the registry.
type ModelEntry struct {
	DisplayName     string          `json:"display_name"`
	Description     *string         `json:"description"`
	OwnedBy         *string         `json:"owned_by"`
	Family          *string         `json:"family"`
	ReleaseDate     *string         `json:"release_date"`
	DeprecationDate *string         `json:"deprecation_date"`
	Tags            []string        `json:"tags"`
	Mode            string          `json:"mode"`
	Modalities      *Modalities     `json:"modalities"`
	Capabilities    map[string]bool `json:"capabilities"`
	ContextWindow   *int            `json:"context_window"`
	MaxOutputTokens *int            `json:"max_output_tokens"`
	Pricing         *PricingEntry   `json:"pricing"`
}

// ProviderModelEntry represents a provider-specific model override.
type ProviderModelEntry struct {
	ModelRef        string       `json:"model_ref"`
	ProviderModelID *string      `json:"provider_model_id"`
	Enabled         bool         `json:"enabled"`
	Pricing         *PricingEntry `json:"pricing"`
	ContextWindow   *int         `json:"context_window"`
	MaxOutputTokens *int         `json:"max_output_tokens"`
	RateLimits      *RateLimits  `json:"rate_limits"`
	Endpoints       []string     `json:"endpoints"`
	Regions         []string     `json:"regions"`
}

// PricingEntry represents pricing information from the registry.
type PricingEntry struct {
	Currency              string   `json:"currency"`
	InputPerMtok          *float64 `json:"input_per_mtok"`
	OutputPerMtok         *float64 `json:"output_per_mtok"`
	CachedInputPerMtok    *float64 `json:"cached_input_per_mtok"`
	ReasoningOutputPerMtok *float64 `json:"reasoning_output_per_mtok"`
	PerImage              *float64 `json:"per_image"`
	PerSecondInput        *float64 `json:"per_second_input"`
	PerSecondOutput       *float64 `json:"per_second_output"`
	PerCharacterInput     *float64 `json:"per_character_input"`
	PerRequest            *float64 `json:"per_request"`
	PerPage               *float64 `json:"per_page"`
}

// Modalities describes input/output modality support.
type Modalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

// RateLimits holds rate limit information.
type RateLimits struct {
	RPM *int `json:"rpm"`
	TPM *int `json:"tpm"`
	RPD *int `json:"rpd"`
}
