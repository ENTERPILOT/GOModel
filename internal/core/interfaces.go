// Package core defines the core interfaces and types for the LLM gateway.
package core

import (
	"context"
	"io"
)

// Provider defines the interface for LLM providers
type Provider interface {
	// ChatCompletion executes a chat completion request
	ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// StreamChatCompletion returns a raw SSE stream (caller must close)
	StreamChatCompletion(ctx context.Context, req *ChatRequest) (io.ReadCloser, error)

	// ListModels returns the list of available models
	ListModels(ctx context.Context) (*ModelsResponse, error)

	// Responses executes a Responses API request (OpenAI-compatible)
	Responses(ctx context.Context, req *ResponsesRequest) (*ResponsesResponse, error)

	// StreamResponses returns a raw SSE stream for Responses API (caller must close)
	StreamResponses(ctx context.Context, req *ResponsesRequest) (io.ReadCloser, error)
}

// RoutableProvider extends Provider with routing capability.
// This is implemented by the Router which uses a model registry
// to determine if a model is supported.
type RoutableProvider interface {
	Provider

	// Supports returns true if the provider can handle the given model
	Supports(model string) bool

	// GetProviderType returns the provider type string for the given model.
	// Returns empty string if the model is not found.
	GetProviderType(model string) string
}
