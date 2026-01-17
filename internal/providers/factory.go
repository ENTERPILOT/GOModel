// Package providers provides a factory for creating provider instances.
package providers

import (
	"fmt"
	"sync"

	"gomodel/config"
	"gomodel/internal/core"
	"gomodel/internal/llmclient"
)

// Builder creates a provider instance from configuration and hooks
type Builder func(cfg config.ProviderConfig, hooks llmclient.Hooks) (core.Provider, error)

// ProviderFactory manages provider registration and creation.
// It replaces the global registry pattern with explicit dependency injection.
type ProviderFactory struct {
	mu       sync.RWMutex
	builders map[string]Builder
	hooks    llmclient.Hooks
}

// NewProviderFactory creates a new provider factory instance.
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		builders: make(map[string]Builder),
	}
}

// SetHooks configures observability hooks that will be passed to all providers.
// This must be called before Create() to take effect.
func (f *ProviderFactory) SetHooks(hooks llmclient.Hooks) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hooks = hooks
}

// GetHooks returns the currently configured hooks.
func (f *ProviderFactory) GetHooks() llmclient.Hooks {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.hooks
}

// Register adds a provider builder to the factory.
func (f *ProviderFactory) Register(providerType string, builder Builder) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.builders[providerType] = builder
}

// RegisterProvider registers a provider constructor with base URL support.
// This is a convenience method that wraps simple constructors.
func RegisterProvider[T core.Provider](f *ProviderFactory, providerType string, newProvider func(apiKey string, hooks llmclient.Hooks) T) {
	f.Register(providerType, func(cfg config.ProviderConfig, hooks llmclient.Hooks) (core.Provider, error) {
		p := newProvider(cfg.APIKey, hooks)
		if cfg.BaseURL != "" {
			if setter, ok := any(p).(interface{ SetBaseURL(string) }); ok {
				setter.SetBaseURL(cfg.BaseURL)
			}
		}
		return p, nil
	})
}

// Create instantiates a provider based on configuration.
func (f *ProviderFactory) Create(cfg config.ProviderConfig) (core.Provider, error) {
	f.mu.RLock()
	builder, ok := f.builders[cfg.Type]
	hooks := f.hooks
	f.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}
	return builder(cfg, hooks)
}

// ListRegistered returns a list of all registered provider types.
func (f *ProviderFactory) ListRegistered() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	types := make([]string, 0, len(f.builders))
	for t := range f.builders {
		types = append(types, t)
	}
	return types
}
