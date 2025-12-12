// Package providers provides model registry and routing for LLM providers.
package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"gomodel/internal/core"
)

// ModelInfo holds information about a model and its provider
type ModelInfo struct {
	Model    core.Model
	Provider core.Provider
}

// CachedModel represents a model stored in the cache file.
// We store provider type to re-associate with the correct provider on load.
type CachedModel struct {
	ProviderType string `json:"provider_type"`
	Object       string `json:"object"`
	OwnedBy      string `json:"owned_by"`
	Created      int64  `json:"created"`
}

// ModelCache represents the cache file structure.
// Models are stored as a map keyed by model ID for direct lookup.
type ModelCache struct {
	Version   int                    `json:"version"`
	UpdatedAt time.Time              `json:"updated_at"`
	Models    map[string]CachedModel `json:"models"`
}

// ModelRegistry manages the mapping of models to their providers.
// It fetches models from providers on startup and caches them in memory.
// Supports loading from a cache file for instant startup.
type ModelRegistry struct {
	mu            sync.RWMutex
	models        map[string]*ModelInfo // model ID -> model info
	providers     []core.Provider
	providerTypes map[core.Provider]string // provider -> type string
	cacheFile     string                   // path to cache file
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

// SetCacheFile sets the path to the cache file for persistent model storage
func (r *ModelRegistry) SetCacheFile(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cacheFile = path
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

// LoadFromCache loads the model list from the cache file.
// Returns the number of models loaded and any error encountered.
func (r *ModelRegistry) LoadFromCache() (int, error) {
	r.mu.RLock()
	cacheFile := r.cacheFile
	r.mu.RUnlock()

	if cacheFile == "" {
		return 0, nil
	}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // No cache file yet, not an error
		}
		return 0, fmt.Errorf("failed to read cache file: %w", err)
	}

	var cache ModelCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return 0, fmt.Errorf("failed to parse cache file: %w", err)
	}

	// Build a map of provider type -> provider for lookup
	r.mu.RLock()
	typeToProvider := make(map[string]core.Provider)
	for provider, pType := range r.providerTypes {
		typeToProvider[pType] = provider
	}
	r.mu.RUnlock()

	// Populate the models map from cache (direct map iteration)
	newModels := make(map[string]*ModelInfo, len(cache.Models))
	for modelID, cached := range cache.Models {
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
		"cache_updated_at", cache.UpdatedAt,
	)

	return len(newModels), nil
}

// SaveToCache saves the current model list to the cache file.
func (r *ModelRegistry) SaveToCache() error {
	r.mu.RLock()
	cacheFile := r.cacheFile
	models := make(map[string]*ModelInfo, len(r.models))
	for k, v := range r.models {
		models[k] = v
	}
	providerTypes := make(map[core.Provider]string, len(r.providerTypes))
	for k, v := range r.providerTypes {
		providerTypes[k] = v
	}
	r.mu.RUnlock()

	if cacheFile == "" {
		return nil
	}

	// Build cache structure (map keyed by model ID)
	cache := ModelCache{
		Version:   1,
		UpdatedAt: time.Now().UTC(),
		Models:    make(map[string]CachedModel, len(models)),
	}

	for modelID, info := range models {
		pType, ok := providerTypes[info.Provider]
		if !ok {
			// Skip models without a known provider type
			continue
		}
		cache.Models[modelID] = CachedModel{
			ProviderType: pType,
			Object:       info.Model.Object,
			OwnedBy:      info.Model.OwnedBy,
			Created:      info.Model.Created,
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(cacheFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	// Write atomically using temp file + rename
	tmpFile := cacheFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}
	if err := os.Rename(tmpFile, cacheFile); err != nil {
		os.Remove(tmpFile) // Clean up temp file
		return fmt.Errorf("failed to rename cache file: %w", err)
	}

	slog.Debug("saved models to cache", "models", len(cache.Models), "file", cacheFile)
	return nil
}

// InitializeAsync starts model fetching in a background goroutine.
// It first loads any cached models for immediate availability, then refreshes from network.
// Returns immediately after loading cache. The background goroutine will update models
// and save to cache when network fetch completes.
func (r *ModelRegistry) InitializeAsync(ctx context.Context) {
	// First, try to load from cache for instant startup
	cached, err := r.LoadFromCache()
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
		if err := r.SaveToCache(); err != nil {
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
func (r *ModelRegistry) GetProvider(modelID string) core.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if info, ok := r.models[modelID]; ok {
		return info.Provider
	}
	return nil
}

// GetModel returns the model info for the given model ID, or nil if not found
func (r *ModelRegistry) GetModel(modelID string) *ModelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if info, ok := r.models[modelID]; ok {
		return info
	}
	return nil
}

// Supports returns true if the registry has a provider for the given model
func (r *ModelRegistry) Supports(modelID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.models[modelID]
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
					if err := r.SaveToCache(); err != nil {
						slog.Warn("failed to save models to cache after refresh", "error", err)
					}
				}
				refreshCancel()
			}
		}
	}()

	return cancel
}
