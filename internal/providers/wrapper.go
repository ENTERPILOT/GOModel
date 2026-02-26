package providers

import (
	"context"
	"io"

	"gomodel/internal/core"
)

type providerWrapper struct {
	inner        core.Provider
	providerName string
}

func newProviderWrapper(p core.Provider, providerName string) core.Provider {
	return &providerWrapper{inner: p, providerName: providerName}
}

func (w *providerWrapper) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	resp, err := w.inner.ChatCompletion(ctx, req)
	if err == nil && resp != nil {
		resp.Provider = w.providerName
	}
	return resp, err
}

func (w *providerWrapper) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	return w.inner.StreamChatCompletion(ctx, req)
}

func (w *providerWrapper) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	return w.inner.ListModels(ctx)
}

func (w *providerWrapper) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	resp, err := w.inner.Responses(ctx, req)
	if err == nil && resp != nil {
		resp.Provider = w.providerName
	}
	return resp, err
}

func (w *providerWrapper) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	return w.inner.StreamResponses(ctx, req)
}

func (w *providerWrapper) Embeddings(ctx context.Context, req *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	resp, err := w.inner.Embeddings(ctx, req)
	if err == nil && resp != nil {
		resp.Provider = w.providerName
	}
	return resp, err
}
