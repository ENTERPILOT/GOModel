package modeldata

import (
	"gomodel/internal/core"
)

// Resolve performs the three-layer merge to produce ModelMetadata for a given
// provider type and model ID. It looks up provider_models[providerType/modelID]
// first, then falls back to models[modelID]. Provider-model fields override
// base model fields where set.
// Returns nil if no match is found in the registry.
func Resolve(list *ModelList, providerType string, modelID string) *core.ModelMetadata {
	if list == nil {
		return nil
	}

	// Try provider_model lookup first: "providerType/modelID"
	var pm *ProviderModelEntry
	key := providerType + "/" + modelID
	if entry, ok := list.ProviderModels[key]; ok {
		pm = &entry
	}

	// Determine the base model entry
	var model *ModelEntry
	if pm != nil {
		// Use model_ref from provider_model to find the base model
		if entry, ok := list.Models[pm.ModelRef]; ok {
			model = &entry
		}
	} else {
		// Fall back to direct model ID lookup
		if entry, ok := list.Models[modelID]; ok {
			model = &entry
		}
	}

	// No match at all
	if model == nil && pm == nil {
		return nil
	}

	meta := &core.ModelMetadata{}

	// Apply base model fields
	if model != nil {
		meta.DisplayName = model.DisplayName
		if model.Description != nil {
			meta.Description = *model.Description
		}
		if model.Family != nil {
			meta.Family = *model.Family
		}
		meta.Mode = model.Mode
		meta.Tags = model.Tags
		meta.ContextWindow = model.ContextWindow
		meta.MaxOutputTokens = model.MaxOutputTokens
		meta.Capabilities = model.Capabilities
		meta.Pricing = convertPricing(model.Pricing)
	}

	// Apply provider_model overrides (non-nil fields win)
	if pm != nil {
		if pm.ContextWindow != nil {
			meta.ContextWindow = pm.ContextWindow
		}
		if pm.MaxOutputTokens != nil {
			meta.MaxOutputTokens = pm.MaxOutputTokens
		}
		if pm.Pricing != nil {
			meta.Pricing = convertPricing(pm.Pricing)
		}
	}

	return meta
}

// convertPricing maps a registry PricingEntry to the core ModelPricing type.
// Currently maps 4 of PricingEntry's 11 fields (Currency, InputPerMtok,
// OutputPerMtok, CachedInputPerMtok). usage.CalculateCost only uses
// InputPerMtok and OutputPerMtok for cost estimation.
// TODO: Extend core.ModelPricing before mapping the remaining PricingEntry
// fields (ReasoningOutputPerMtok, PerImage, PerSecondInput, PerSecondOutput,
// PerCharacterInput, PerRequest, PerPage).
func convertPricing(p *PricingEntry) *core.ModelPricing {
	if p == nil {
		return nil
	}
	return &core.ModelPricing{
		Currency:           p.Currency,
		InputPerMtok:       p.InputPerMtok,
		OutputPerMtok:      p.OutputPerMtok,
		CachedInputPerMtok: p.CachedInputPerMtok,
	}
}
