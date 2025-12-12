// Package providers provides a router for multiple LLM providers.
package providers

import (
	"context"
	"fmt"
	"io"

	"gomodel/internal/core"
)

// Router routes requests to the appropriate provider based on the model registry.
// It uses a dynamic model-to-provider mapping that is populated at startup
// by fetching available models from each provider's /models endpoint.
type Router struct {
	registry *ModelRegistry
}

// NewRouter creates a new provider router with a model registry.
// The registry must be initialized before using the router.
func NewRouter(registry *ModelRegistry) *Router {
	return &Router{
		registry: registry,
	}
}

// Supports returns true if any provider supports the given model
func (r *Router) Supports(model string) bool {
	return r.registry.Supports(model)
}

// ChatCompletion routes the request to the appropriate provider
func (r *Router) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	provider := r.registry.GetProvider(req.Model)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for model: %s", req.Model)
	}
	return provider.ChatCompletion(ctx, req)
}

// StreamChatCompletion routes the streaming request to the appropriate provider
func (r *Router) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	provider := r.registry.GetProvider(req.Model)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for model: %s", req.Model)
	}
	return provider.StreamChatCompletion(ctx, req)
}

// ListModels returns all models from the registry
func (r *Router) ListModels(_ context.Context) (*core.ModelsResponse, error) {
	models := r.registry.ListModels()
	return &core.ModelsResponse{
		Object: "list",
		Data:   models,
	}, nil
}

// Responses routes the Responses API request to the appropriate provider
func (r *Router) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	provider := r.registry.GetProvider(req.Model)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for model: %s", req.Model)
	}
	return provider.Responses(ctx, req)
}

// StreamResponses routes the streaming Responses API request to the appropriate provider
func (r *Router) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	provider := r.registry.GetProvider(req.Model)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for model: %s", req.Model)
	}
	return provider.StreamResponses(ctx, req)
}
