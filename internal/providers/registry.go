// Package providers provides model registry and routing for LLM providers.
package providers

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"gomodel/internal/cache"
	"gomodel/internal/core"
)

// ModelInfo holds information about a model and its provider
type ModelInfo struct {
	Model    core.Model
	Provider core.Provider
}

// ModelRegistry manages the mapping of models to their providers.
// It fetches models from providers on startup and caches them in memory.
// Supports loading from a cache (local file or Redis) for instant startup.
type ModelRegistry struct {
	mu            sync.RWMutex
	models        map[string]*ModelInfo // model ID -> model info
	providers     []core.Provider
	providerTypes map[core.Provider]string // provider -> type string
	cache         cache.Cache              // cache backend (local or redis)
	initialized   bool                     // true when at least one successful network fetch completed
	initMu        sync.Mutex               // protects initialized flag
}

// NewModelRegistry creates a new model registry
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		models:        make(map[string]*ModelInfo),
		providerTypes: make(map[core.Provider]string),
	}
}

// SetCache sets the cache backend for persistent model storage.
// The cache can be a local file-based cache or a Redis cache.
func (r *ModelRegistry) SetCache(c cache.Cache) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = c
}

// RegisterProvider adds a provider to the registry
func (r *ModelRegistry) RegisterProvider(provider core.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, provider)
}

// RegisterProviderWithType adds a provider to the registry with its type string.
// The type is used for cache persistence to re-associate models with providers on startup.
func (r *ModelRegistry) RegisterProviderWithType(provider core.Provider, providerType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, provider)
	r.providerTypes[provider] = providerType
}

// Initialize fetches models from all registered providers and populates the registry.
// This should be called on application startup.
func (r *ModelRegistry) Initialize(ctx context.Context) error {
	// Get a snapshot of providers with a read lock
	r.mu.RLock()
	providers := make([]core.Provider, len(r.providers))
	copy(providers, r.providers)
	r.mu.RUnlock()

	// Build new models map without holding the lock.
	// This allows concurrent reads to continue using the existing map
	// while we fetch models from providers (which may involve network calls).
	newModels := make(map[string]*ModelInfo)
	var totalModels int
	var failedProviders int

	for _, provider := range providers {
		resp, err := provider.ListModels(ctx)
		if err != nil {
			slog.Warn("failed to fetch models from provider",
				"error", err,
			)
			failedProviders++
			continue
		}

		for _, model := range resp.Data {
			if _, exists := newModels[model.ID]; exists {
				// Model already registered by another provider, skip
				// First provider wins (could be made configurable)
				slog.Debug("model already registered, skipping",
					"model", model.ID,
					"owner", model.OwnedBy,
				)
				continue
			}

			newModels[model.ID] = &ModelInfo{
				Model:    model,
				Provider: provider,
			}
			totalModels++
		}
	}

	if totalModels == 0 {
		if failedProviders == len(providers) {
			return fmt.Errorf("failed to fetch models from any provider")
		}
		return fmt.Errorf("no models available: providers returned empty model lists")
	}

	// Atomically swap the models map
	r.mu.Lock()
	r.models = newModels
	r.mu.Unlock()

	// Mark as initialized
	r.initMu.Lock()
	r.initialized = true
	r.initMu.Unlock()

	slog.Info("model registry initialized",
		"total_models", totalModels,
		"providers", len(providers),
		"failed_providers", failedProviders,
	)

	return nil
}

// Refresh updates the model registry by fetching fresh model lists from providers.
// This can be called periodically to keep the registry up to date.
func (r *ModelRegistry) Refresh(ctx context.Context) error {
	return r.Initialize(ctx)
}

// LoadFromCache loads the model list from the cache backend.
// Returns the number of models loaded and any error encountered.
func (r *ModelRegistry) LoadFromCache(ctx context.Context) (int, error) {
	r.mu.RLock()
	cacheBackend := r.cache
	r.mu.RUnlock()

	if cacheBackend == nil {
		return 0, nil
	}

	modelCache, err := cacheBackend.Get(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to read cache: %w", err)
	}

	if modelCache == nil {
		return 0, nil // No cache yet, not an error
	}

	// Build a map of provider type -> provider for lookup
	r.mu.RLock()
	typeToProvider := make(map[string]core.Provider)
	for provider, pType := range r.providerTypes {
		typeToProvider[pType] = provider
	}
	r.mu.RUnlock()

	// Populate the models map from cache (direct map iteration)
	newModels := make(map[string]*ModelInfo, len(modelCache.Models))
	for modelID, cached := range modelCache.Models {
		provider, ok := typeToProvider[cached.ProviderType]
		if !ok {
			// Provider not configured, skip this model
			continue
		}
		newModels[modelID] = &ModelInfo{
			Model: core.Model{
				ID:      modelID,
				Object:  cached.Object,
				OwnedBy: cached.OwnedBy,
				Created: cached.Created,
			},
			Provider: provider,
		}
	}

	r.mu.Lock()
	r.models = newModels
	r.mu.Unlock()

	slog.Info("loaded models from cache",
		"models", len(newModels),
		"cache_updated_at", modelCache.UpdatedAt,
	)

	return len(newModels), nil
}

// SaveToCache saves the current model list to the cache backend.
func (r *ModelRegistry) SaveToCache(ctx context.Context) error {
	r.mu.RLock()
	cacheBackend := r.cache
	models := make(map[string]*ModelInfo, len(r.models))
	for k, v := range r.models {
		models[k] = v
	}
	providerTypes := make(map[core.Provider]string, len(r.providerTypes))
	for k, v := range r.providerTypes {
		providerTypes[k] = v
	}
	r.mu.RUnlock()

	if cacheBackend == nil {
		return nil
	}

	// Build cache structure (map keyed by model ID)
	modelCache := &cache.ModelCache{
		Version:   1,
		UpdatedAt: time.Now().UTC(),
		Models:    make(map[string]cache.CachedModel, len(models)),
	}

	for modelID, info := range models {
		pType, ok := providerTypes[info.Provider]
		if !ok {
			// Skip models without a known provider type
			continue
		}
		modelCache.Models[modelID] = cache.CachedModel{
			ProviderType: pType,
			Object:       info.Model.Object,
			OwnedBy:      info.Model.OwnedBy,
			Created:      info.Model.Created,
		}
	}

	if err := cacheBackend.Set(ctx, modelCache); err != nil {
		return fmt.Errorf("failed to save cache: %w", err)
	}

	slog.Debug("saved models to cache", "models", len(modelCache.Models))
	return nil
}

// InitializeAsync starts model fetching in a background goroutine.
// It first loads any cached models for immediate availability, then refreshes from network.
// Returns immediately after loading cache. The background goroutine will update models
// and save to cache when network fetch completes.
func (r *ModelRegistry) InitializeAsync(ctx context.Context) {
	// First, try to load from cache for instant startup
	cached, err := r.LoadFromCache(ctx)
	if err != nil {
		slog.Warn("failed to load models from cache", "error", err)
	} else if cached > 0 {
		slog.Info("serving traffic with cached models while refreshing", "cached_models", cached)
	}

	// Start background initialization
	go func() {
		initCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		if err := r.Initialize(initCtx); err != nil {
			slog.Warn("background model initialization failed", "error", err)
			return
		}

		// Save to cache for next startup
		if err := r.SaveToCache(initCtx); err != nil {
			slog.Warn("failed to save models to cache", "error", err)
		}
	}()
}

// IsInitialized returns true if at least one successful network fetch has completed.
// This can be used to check if the registry has fresh data or is only serving from cache.
func (r *ModelRegistry) IsInitialized() bool {
	r.initMu.Lock()
	defer r.initMu.Unlock()
	return r.initialized
}

// GetProvider returns the provider for the given model, or nil if not found
func (r *ModelRegistry) GetProvider(model string) core.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if info, ok := r.models[model]; ok {
		return info.Provider
	}
	return nil
}

// GetModel returns the model info for the given model, or nil if not found
func (r *ModelRegistry) GetModel(model string) *ModelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if info, ok := r.models[model]; ok {
		return info
	}
	return nil
}

// Supports returns true if the registry has a provider for the given model
func (r *ModelRegistry) Supports(model string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.models[model]
	return ok
}

// ListModels returns all models in the registry, sorted by model ID for consistent ordering.
func (r *ModelRegistry) ListModels() []core.Model {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]core.Model, 0, len(r.models))
	for _, info := range r.models {
		models = append(models, info.Model)
	}

	// Sort by model ID for consistent ordering across calls
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	return models
}

// ModelCount returns the number of registered models
func (r *ModelRegistry) ModelCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.models)
}

// GetProviderType returns the provider type string for the given model.
// Returns empty string if the model is not found.
func (r *ModelRegistry) GetProviderType(model string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, ok := r.models[model]
	if !ok {
		return ""
	}

	return r.providerTypes[info.Provider]
}

// ProviderCount returns the number of registered providers
func (r *ModelRegistry) ProviderCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.providers)
}

// StartBackgroundRefresh starts a goroutine that periodically refreshes the model registry.
// Returns a cancel function to stop the refresh loop.
func (r *ModelRegistry) StartBackgroundRefresh(interval time.Duration) func() {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				refreshCtx, refreshCancel := context.WithTimeout(context.Background(), 30*time.Second)
				if err := r.Refresh(refreshCtx); err != nil {
					slog.Warn("background model refresh failed", "error", err)
				} else {
					// Save to cache after successful refresh
					if err := r.SaveToCache(refreshCtx); err != nil {
						slog.Warn("failed to save models to cache after refresh", "error", err)
					}
				}
				refreshCancel()
			}
		}
	}()

	return cancel
}
