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
		meta.Pricing = model.Pricing
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
			meta.Pricing = pm.Pricing
		}
	}

	return meta
}
