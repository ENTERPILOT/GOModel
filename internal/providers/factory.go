// Package providers provides a factory for creating provider instances.
package providers

import (
	"fmt"
	"sync"

	"gomodel/config"
	"gomodel/internal/core"
	"gomodel/internal/llmclient"
)

// NewFunc is the constructor signature for providers.
type NewFunc func(apiKey string, hooks llmclient.Hooks) core.Provider

// Registration contains metadata for registering a provider with the factory.
type Registration struct {
	// Type is the provider identifier (e.g., "openai", "anthropic")
	Type string

	// New creates a new provider instance
	New NewFunc
}

// ProviderFactory manages provider registration and creation.
type ProviderFactory struct {
	mu       sync.RWMutex
	builders map[string]NewFunc
	hooks    llmclient.Hooks
}

// NewProviderFactory creates a new provider factory instance.
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		builders: make(map[string]NewFunc),
	}
}

// SetHooks configures observability hooks for all providers created by this factory.
func (f *ProviderFactory) SetHooks(hooks llmclient.Hooks) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hooks = hooks
}

// Register adds a provider to the factory.
func (f *ProviderFactory) Register(reg Registration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.builders[reg.Type] = reg.New
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

	p := builder(cfg.APIKey, hooks)

	// Set custom base URL if configured
	if cfg.BaseURL != "" {
		if setter, ok := p.(interface{ SetBaseURL(string) }); ok {
			setter.SetBaseURL(cfg.BaseURL)
		}
	}

	return p, nil
}

// RegisteredTypes returns a list of all registered provider types.
func (f *ProviderFactory) RegisteredTypes() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	types := make([]string, 0, len(f.builders))
	for t := range f.builders {
		types = append(types, t)
	}
	return types
}
