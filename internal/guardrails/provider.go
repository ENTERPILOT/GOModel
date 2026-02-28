package guardrails

import (
	"context"
	"io"

	"gomodel/internal/core"
)

// GuardedProvider wraps a RoutableProvider and applies the guardrails pipeline
// before routing requests to providers. It implements core.RoutableProvider.
//
// Adapters convert between concrete request types and the normalized []Message
// DTO that guardrails operate on. This decouples guardrails from API-specific types.
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

// ChatCompletion extracts messages, applies guardrails, then routes the request.
func (g *GuardedProvider) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	modified, err := g.processChat(ctx, req)
	if err != nil {
		return nil, err
	}
	return g.inner.ChatCompletion(ctx, modified)
}

// StreamChatCompletion extracts messages, applies guardrails, then routes the streaming request.
func (g *GuardedProvider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	modified, err := g.processChat(ctx, req)
	if err != nil {
		return nil, err
	}
	return g.inner.StreamChatCompletion(ctx, modified)
}

// ListModels delegates directly to the inner provider (no guardrails needed).
func (g *GuardedProvider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	return g.inner.ListModels(ctx)
}

// Embeddings delegates directly to the inner provider (no guardrails needed for embeddings).
func (g *GuardedProvider) Embeddings(ctx context.Context, req *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	return g.inner.Embeddings(ctx, req)
}

// Responses extracts messages, applies guardrails, then routes the request.
func (g *GuardedProvider) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	modified, err := g.processResponses(ctx, req)
	if err != nil {
		return nil, err
	}
	return g.inner.Responses(ctx, modified)
}

// StreamResponses extracts messages, applies guardrails, then routes the streaming request.
func (g *GuardedProvider) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	modified, err := g.processResponses(ctx, req)
	if err != nil {
		return nil, err
	}
	return g.inner.StreamResponses(ctx, modified)
}

// processChat runs the pipeline for a ChatRequest via the message adapter.
func (g *GuardedProvider) processChat(ctx context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
	msgs := chatToMessages(req)
	modified, err := g.pipeline.Process(ctx, msgs)
	if err != nil {
		return nil, err
	}
	return applyMessagesToChat(req, modified), nil
}

// processResponses runs the pipeline for a ResponsesRequest via the message adapter.
func (g *GuardedProvider) processResponses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesRequest, error) {
	msgs := responsesToMessages(req)
	modified, err := g.pipeline.Process(ctx, msgs)
	if err != nil {
		return nil, err
	}
	return applyMessagesToResponses(req, modified), nil
}

// --- Adapters: concrete requests ↔ normalized []Message ---

// chatToMessages extracts the normalized message list from a ChatRequest.
func chatToMessages(req *core.ChatRequest) []Message {
	msgs := make([]Message, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = Message{Role: m.Role, Content: m.Content}
	}
	return msgs
}

// applyMessagesToChat returns a shallow copy of req with messages replaced.
func applyMessagesToChat(req *core.ChatRequest, msgs []Message) *core.ChatRequest {
	coreMessages := make([]core.Message, len(msgs))
	for i, m := range msgs {
		coreMessages[i] = core.Message{Role: m.Role, Content: m.Content}
	}
	result := *req
	result.Messages = coreMessages
	return &result
}

// responsesToMessages extracts the normalized message list from a ResponsesRequest.
// The Instructions field maps to a system message.
func responsesToMessages(req *core.ResponsesRequest) []Message {
	var msgs []Message
	if req.Instructions != "" {
		msgs = append(msgs, Message{Role: "system", Content: req.Instructions})
	}
	return msgs
}

// applyMessagesToResponses returns a shallow copy of req with system messages
// applied back to the Instructions field.
func applyMessagesToResponses(req *core.ResponsesRequest, msgs []Message) *core.ResponsesRequest {
	result := *req
	var instructions string
	for _, m := range msgs {
		if m.Role == "system" {
			if instructions != "" {
				instructions += "\n"
			}
			instructions += m.Content
		}
	}
	result.Instructions = instructions
	return &result
}
