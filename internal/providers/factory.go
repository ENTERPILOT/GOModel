// Package providers provides a factory for creating provider instances.
package providers

import (
	"fmt"

	"gomodel/config"
	"gomodel/internal/core"
)

// Builder creates a provider instance from configuration
type Builder func(cfg config.ProviderConfig) (core.Provider, error)

// registry holds all registered provider builders
var registry = make(map[string]Builder)

// Register allows provider packages to register themselves
// This should be called from init() functions in provider packages
func Register(providerType string, builder Builder) {
	registry[providerType] = builder
}

// RegisterProvider registers a provider constructor with base URL support
func RegisterProvider[T core.Provider](providerType string, newProvider func(string) T) {
	Register(providerType, func(cfg config.ProviderConfig) (core.Provider, error) {
		p := newProvider(cfg.APIKey)
		if cfg.BaseURL != "" {
			if setter, ok := any(p).(interface{ SetBaseURL(string) }); ok {
				setter.SetBaseURL(cfg.BaseURL)
			}
		}
		return p, nil
	})
}

// Create instantiates a provider based on configuration
func Create(cfg config.ProviderConfig) (core.Provider, error) {
	builder, ok := registry[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}
	return builder(cfg)
}

// ListRegistered returns a list of all registered provider types
func ListRegistered() []string {
	types := make([]string, 0, len(registry))
	for t := range registry {
		types = append(types, t)
	}
	return types
}
