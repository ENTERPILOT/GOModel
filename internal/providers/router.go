// Package providers provides a router for multiple LLM providers.
package providers

import (
	"context"
	"fmt"
	"io"

	"gomodel/internal/core"
)

// Router routes requests to the appropriate provider based on the model
type Router struct {
	providers []core.Provider
}

// NewRouter creates a new provider router
func NewRouter(providers ...core.Provider) *Router {
	return &Router{
		providers: providers,
	}
}

// Supports returns true if any provider supports the given model
func (r *Router) Supports(model string) bool {
	for _, p := range r.providers {
		if p.Supports(model) {
			return true
		}
	}
	return false
}

// ChatCompletion routes the request to the appropriate provider
func (r *Router) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	provider := r.findProvider(req.Model)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for model: %s", req.Model)
	}
	return provider.ChatCompletion(ctx, req)
}

// StreamChatCompletion routes the streaming request to the appropriate provider
func (r *Router) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	provider := r.findProvider(req.Model)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for model: %s", req.Model)
	}
	return provider.StreamChatCompletion(ctx, req)
}

// ListModels returns models from all providers
func (r *Router) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	allModels := make([]core.Model, 0)

	for _, p := range r.providers {
		resp, err := p.ListModels(ctx)
		if err != nil {
			// Log error but continue with other providers
			continue
		}
		allModels = append(allModels, resp.Data...)
	}

	return &core.ModelsResponse{
		Object: "list",
		Data:   allModels,
	}, nil
}

// findProvider finds the first provider that supports the given model
func (r *Router) findProvider(model string) core.Provider {
	for _, p := range r.providers {
		if p.Supports(model) {
			return p
		}
	}
	return nil
}
