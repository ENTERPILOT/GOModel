package core

import (
	"fmt"
	"strings"
)

// ModelSelector is a normalized model routing selector.
// Model is always the raw upstream model ID (without provider prefix).
type ModelSelector struct {
	Model    string
	Provider string
}

// QualifiedModel returns "provider/model" when Provider is set, or only model otherwise.
func (s ModelSelector) QualifiedModel() string {
	if s.Provider == "" {
		return s.Model
	}
	return s.Provider + "/" + s.Model
}

// ParseModelSelector normalizes model/provider routing input.
//
// Accepted forms:
//   - model only: "gpt-4o"
//   - model with prefix: "openai/gpt-4o"
//   - explicit provider field: provider="openai", model="gpt-4o"
//
// If provider is present in both places, values must match.
func ParseModelSelector(model, provider string) (ModelSelector, error) {
	model = strings.TrimSpace(model)
	provider = strings.TrimSpace(provider)

	if model == "" {
		return ModelSelector{}, fmt.Errorf("model is required")
	}

	parts := strings.SplitN(model, "/", 2)
	if len(parts) == 2 {
		prefix := strings.TrimSpace(parts[0])
		rest := strings.TrimSpace(parts[1])
		if prefix != "" && rest != "" {
			if provider != "" && provider != prefix {
				return ModelSelector{}, fmt.Errorf("provider field %q conflicts with model prefix %q", provider, prefix)
			}
			provider = prefix
			model = rest
		}
	}

	if model == "" {
		return ModelSelector{}, fmt.Errorf("model is required")
	}

	return ModelSelector{
		Model:    model,
		Provider: provider,
	}, nil
}
