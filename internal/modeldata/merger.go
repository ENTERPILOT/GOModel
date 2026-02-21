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

	// No match at all — try reverse lookup via provider_model_id
	if model == nil && pm == nil {
		if list.providerModelByActualID != nil {
			reverseKey := providerType + "/" + modelID
			if compositeKey, ok := list.providerModelByActualID[reverseKey]; ok {
				return Resolve(list, providerType, compositeKey[len(providerType)+1:])
			}
		}
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
		meta.Modes = model.Modes
		meta.Categories = core.CategoriesForModes(model.Modes)
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
func convertPricing(p *PricingEntry) *core.ModelPricing {
	if p == nil {
		return nil
	}
	return &core.ModelPricing{
		Currency:               p.Currency,
		InputPerMtok:           p.InputPerMtok,
		OutputPerMtok:          p.OutputPerMtok,
		CachedInputPerMtok:     p.CachedInputPerMtok,
		CacheWritePerMtok:      p.CacheWritePerMtok,
		ReasoningOutputPerMtok: p.ReasoningOutputPerMtok,
		BatchInputPerMtok:      p.BatchInputPerMtok,
		BatchOutputPerMtok:     p.BatchOutputPerMtok,
		AudioInputPerMtok:      p.AudioInputPerMtok,
		AudioOutputPerMtok:     p.AudioOutputPerMtok,
		PerImage:               p.PerImage,
		PerSecondInput:         p.PerSecondInput,
		PerSecondOutput:        p.PerSecondOutput,
		PerCharacterInput:      p.PerCharacterInput,
		PerRequest:             p.PerRequest,
		PerPage:                p.PerPage,
		Tiers:                  convertPricingTiers(p.Tiers),
	}
}

// convertPricingTiers maps registry PricingTier entries to core ModelPricingTier.
func convertPricingTiers(tiers []PricingTier) []core.ModelPricingTier {
	if len(tiers) == 0 {
		return nil
	}
	result := make([]core.ModelPricingTier, len(tiers))
	for i, t := range tiers {
		result[i] = core.ModelPricingTier{
			UpToMtok:      t.UpToMtok,
			InputPerMtok:  t.InputPerMtok,
			OutputPerMtok: t.OutputPerMtok,
		}
	}
	return result
}
