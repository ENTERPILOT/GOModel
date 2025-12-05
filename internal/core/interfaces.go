package core

import "context"

// Provider defines the interface for LLM providers
type Provider interface {
	// Supports determines if this provider can handle the model name
	Supports(model string) bool

	// ChatCompletion executes a chat completion request
	ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// StreamChatCompletion executes a streaming chat completion request
	StreamChatCompletion(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error)
}
