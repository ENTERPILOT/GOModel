// Package providers provides model registry and routing for LLM providers.
package providers

import (
	"context"
	"fmt"
	"log/slog"
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

// ModelRegistry manages the mapping of models to their providers.
// It fetches models from providers on startup and caches them in memory.
type ModelRegistry struct {
	mu        sync.RWMutex
	models    map[string]*ModelInfo // model ID -> model info
	providers []core.Provider
}

// NewModelRegistry creates a new model registry
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		models: make(map[string]*ModelInfo),
	}
}

// RegisterProvider adds a provider to the registry
func (r *ModelRegistry) RegisterProvider(provider core.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, provider)
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

	if totalModels == 0 && failedProviders == len(providers) {
		return fmt.Errorf("failed to fetch models from any provider")
	}

	// Atomically swap the models map
	r.mu.Lock()
	r.models = newModels
	r.mu.Unlock()

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
				}
				refreshCancel()
			}
		}
	}()

	return cancel
}
