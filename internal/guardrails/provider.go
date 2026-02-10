package guardrails

import (
	"context"
	"io"

	"gomodel/internal/core"
)

// GuardedProvider wraps a RoutableProvider and applies the guardrails pipeline
// before routing requests to providers. It implements core.RoutableProvider.
type GuardedProvider struct {
	inner    core.RoutableProvider
	pipeline *Pipeline
}

// NewGuardedProvider creates a RoutableProvider that applies guardrails
// before delegating to the inner provider.
func NewGuardedProvider(inner core.RoutableProvider, pipeline *Pipeline) *GuardedProvider {
	return &GuardedProvider{
		inner:    inner,
		pipeline: pipeline,
	}
}

// Supports delegates to the inner provider.
func (g *GuardedProvider) Supports(model string) bool {
	return g.inner.Supports(model)
}

// GetProviderType delegates to the inner provider.
func (g *GuardedProvider) GetProviderType(model string) string {
	return g.inner.GetProviderType(model)
}

// ChatCompletion applies guardrails then routes the request.
func (g *GuardedProvider) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	modified, err := g.pipeline.ProcessChat(ctx, req)
	if err != nil {
		return nil, err
	}
	return g.inner.ChatCompletion(ctx, modified)
}

// StreamChatCompletion applies guardrails then routes the streaming request.
func (g *GuardedProvider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	modified, err := g.pipeline.ProcessChat(ctx, req)
	if err != nil {
		return nil, err
	}
	return g.inner.StreamChatCompletion(ctx, modified)
}

// ListModels delegates directly to the inner provider (no guardrails needed).
func (g *GuardedProvider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	return g.inner.ListModels(ctx)
}

// Responses applies guardrails then routes the request.
func (g *GuardedProvider) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	modified, err := g.pipeline.ProcessResponses(ctx, req)
	if err != nil {
		return nil, err
	}
	return g.inner.Responses(ctx, modified)
}

// StreamResponses applies guardrails then routes the streaming request.
func (g *GuardedProvider) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	modified, err := g.pipeline.ProcessResponses(ctx, req)
	if err != nil {
		return nil, err
	}
	return g.inner.StreamResponses(ctx, modified)
}
