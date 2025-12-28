// Package providers provides a factory for creating provider instances.
package providers

import (
	"fmt"

	"gomodel/config"
	"gomodel/internal/core"
	"gomodel/internal/llmclient"
)

// Builder creates a provider instance from configuration
type Builder func(cfg config.ProviderConfig) (core.Provider, error)

// registry holds all registered provider builders
var registry = make(map[string]Builder)

// globalHooks holds the observability hooks to inject into all providers
var globalHooks llmclient.Hooks

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

// SetGlobalHooks configures observability hooks that will be injected into all providers.
// This must be called before Create() to take effect.
// This enables metrics, tracing, and logging without modifying provider implementations.
func SetGlobalHooks(hooks llmclient.Hooks) {
	globalHooks = hooks
}

// GetGlobalHooks returns the currently configured global hooks
func GetGlobalHooks() llmclient.Hooks {
	return globalHooks
}
