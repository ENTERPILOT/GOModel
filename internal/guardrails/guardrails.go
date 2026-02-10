// Package guardrails provides a pluggable pipeline for request-level guardrails.
//
// Guardrails intercept requests before they reach providers, allowing
// validation, modification, or rejection. They can be executed sequentially
// (each receives the previous guardrail's output) or in parallel (all run
// concurrently on the original request, modifications applied in order).
package guardrails

import (
	"context"

	"gomodel/internal/core"
)

// Guardrail processes a request and returns the (possibly modified) request or an error.
// Returning an error rejects the request before it reaches the provider.
type Guardrail interface {
	// Name returns a human-readable identifier for this guardrail.
	Name() string

	// ProcessChat processes a chat completion request.
	// Return the (possibly modified) request, or an error to reject it.
	ProcessChat(ctx context.Context, req *core.ChatRequest) (*core.ChatRequest, error)

	// ProcessResponses processes a Responses API request.
	// Return the (possibly modified) request, or an error to reject it.
	ProcessResponses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesRequest, error)
}

// ExecutionMode defines how guardrails in a pipeline are executed.
type ExecutionMode string

const (
	// Sequential runs guardrails one after another; each receives the output
	// of the previous guardrail. Order matters.
	Sequential ExecutionMode = "sequential"

	// Parallel runs all guardrails concurrently on the original request.
	// If any guardrail returns an error, the pipeline fails.
	// Modifications are applied in registration order after all complete.
	Parallel ExecutionMode = "parallel"
)
