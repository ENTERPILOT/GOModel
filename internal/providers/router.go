// Package providers provides a router for multiple LLM providers.
package providers

import (
	"context"
	"fmt"
	"io"

	"gomodel/internal/core"
)

// ErrRegistryNotInitialized is returned when the router is used before the registry has any models.
var ErrRegistryNotInitialized = fmt.Errorf("model registry has no models: ensure Initialize() or LoadFromCache() is called before using the router")

// Router routes requests to the appropriate provider based on the model registry.
// It uses a dynamic model-to-provider mapping that is populated at startup
// by fetching available models from each provider's /models endpoint.
type Router struct {
	registry *ModelRegistry
}

// NewRouter creates a new provider router with a model registry.
// The registry must be initialized (via Initialize() or LoadFromCache()) before using the router.
// Returns an error if the registry is nil.
func NewRouter(registry *ModelRegistry) (*Router, error) {
	if registry == nil {
		return nil, fmt.Errorf("registry cannot be nil")
	}
	return &Router{
		registry: registry,
	}, nil
}

// checkReady verifies the registry has models available.
// Returns ErrRegistryNotInitialized if no models are loaded.
func (r *Router) checkReady() error {
	if r.registry.ModelCount() == 0 {
		return ErrRegistryNotInitialized
	}
	return nil
}

// Supports returns true if any provider supports the given model.
// Returns false if the registry has no models loaded.
func (r *Router) Supports(model string) bool {
	if r.registry.ModelCount() == 0 {
		return false
	}
	return r.registry.Supports(model)
}

// ChatCompletion routes the request to the appropriate provider.
// Returns ErrRegistryNotInitialized if the registry has no models loaded.
func (r *Router) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	provider := r.registry.GetProvider(req.Model)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for model: %s", req.Model)
	}
	return provider.ChatCompletion(ctx, req)
}

// StreamChatCompletion routes the streaming request to the appropriate provider.
// Returns ErrRegistryNotInitialized if the registry has no models loaded.
func (r *Router) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	provider := r.registry.GetProvider(req.Model)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for model: %s", req.Model)
	}
	return provider.StreamChatCompletion(ctx, req)
}

// ListModels returns all models from the registry.
// Returns ErrRegistryNotInitialized if the registry has no models loaded.
func (r *Router) ListModels(_ context.Context) (*core.ModelsResponse, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	models := r.registry.ListModels()
	return &core.ModelsResponse{
		Object: "list",
		Data:   models,
	}, nil
}

// Responses routes the Responses API request to the appropriate provider.
// Returns ErrRegistryNotInitialized if the registry has no models loaded.
func (r *Router) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	provider := r.registry.GetProvider(req.Model)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for model: %s", req.Model)
	}
	return provider.Responses(ctx, req)
}

// StreamResponses routes the streaming Responses API request to the appropriate provider.
// Returns ErrRegistryNotInitialized if the registry has no models loaded.
func (r *Router) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	if err := r.checkReady(); err != nil {
		return nil, err
	}
	provider := r.registry.GetProvider(req.Model)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for model: %s", req.Model)
	}
	return provider.StreamResponses(ctx, req)
}

// GetProviderType returns the provider type string for the given model.
// Returns empty string if the model is not found.
func (r *Router) GetProviderType(model string) string {
	return r.registry.GetProviderType(model)
}
