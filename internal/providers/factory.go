// Package providers provides a factory for creating provider instances.
package providers

import (
	"fmt"
	"sort"
	"sync"

	"gomodel/config"
	"gomodel/internal/core"
	"gomodel/internal/llmclient"
)

// ProviderOptions bundles runtime settings passed from the factory to provider constructors.
type ProviderOptions struct {
	Hooks      llmclient.Hooks
	Resilience config.ResilienceConfig
}

// ProviderConstructor is the constructor signature for providers.
type ProviderConstructor func(apiKey string, opts ProviderOptions) core.Provider

// Registration contains metadata for registering a provider with the factory.
type Registration struct {
	Type                string
	New                 ProviderConstructor
	CostMappings        []core.TokenCostMapping // optional: provider-specific token cost mappings
	InformationalFields []string                // optional: known breakdown fields that need no separate pricing
}

// ProviderFactory manages provider registration and creation.
type ProviderFactory struct {
	mu            sync.RWMutex
	registrations map[string]Registration
	hooks         llmclient.Hooks
}

// NewProviderFactory creates a new provider factory instance.
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		registrations: make(map[string]Registration),
	}
}

// SetHooks configures observability hooks for all providers created by this factory.
func (f *ProviderFactory) SetHooks(hooks llmclient.Hooks) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hooks = hooks
}

// Add adds a provider constructor to the factory.
// Panics if reg.Type is empty or reg.New is nil — both are programming errors
// caught at startup, not runtime conditions.
func (f *ProviderFactory) Add(reg Registration) {
	if reg.Type == "" {
		panic("providers: Add called with empty Type")
	}
	if reg.New == nil {
		panic(fmt.Sprintf("providers: Add called with nil constructor for type %q", reg.Type))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registrations[reg.Type] = reg
}

// Create instantiates a provider based on its resolved configuration.
func (f *ProviderFactory) Create(cfg ProviderConfig) (core.Provider, error) {
	f.mu.RLock()
	reg, ok := f.registrations[cfg.Type]
	hooks := f.hooks
	f.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}

	opts := ProviderOptions{
		Hooks:      hooks,
		Resilience: cfg.Resilience,
	}

	p := reg.New(cfg.APIKey, opts)

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

	types := make([]string, 0, len(f.registrations))
	for t := range f.registrations {
		types = append(types, t)
	}
	return types
}

// CostRegistry returns aggregated cost mappings and informational fields from all
// registered providers. The returned map is keyed by provider type.
func (f *ProviderFactory) CostRegistry() (mappings map[string][]core.TokenCostMapping, informationalFields []string) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	mappings = make(map[string][]core.TokenCostMapping)
	seen := make(map[string]struct{})

	for _, reg := range f.registrations {
		if len(reg.CostMappings) > 0 {
			mappings[reg.Type] = reg.CostMappings
		}
		for _, field := range reg.InformationalFields {
			if _, ok := seen[field]; !ok {
				seen[field] = struct{}{}
				informationalFields = append(informationalFields, field)
			}
		}
	}

	sort.Strings(informationalFields)
	return mappings, informationalFields
}
