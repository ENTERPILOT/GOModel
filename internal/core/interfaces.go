// Package core defines the core interfaces and types for the LLM gateway.
package core

import (
	"context"
	"io"
)

// Provider defines the interface for LLM providers
type Provider interface {
	// Supports determines if this provider can handle the model name
	Supports(model string) bool

	// ChatCompletion executes a chat completion request
	ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// StreamChatCompletion returns a raw SSE stream (caller must close)
	StreamChatCompletion(ctx context.Context, req *ChatRequest) (io.ReadCloser, error)

	// ListModels returns the list of available models
	ListModels(ctx context.Context) (*ModelsResponse, error)
}
