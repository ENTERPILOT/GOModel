// Package guardrails provides a pluggable pipeline for request-level guardrails.
//
// Guardrails intercept requests before they reach providers, allowing
// validation, modification, or rejection.
//
// Execution is driven by a per-guardrail "order" value:
//   - Guardrails with the same order run in parallel (concurrently).
//   - Groups are executed sequentially in ascending order.
//   - Each group receives the output of the previous group.
//
// Example with orders 0, 0, 1, 2, 2:
//
//	Group 0  ──┬── guardrail A ──┬──▶ Group 1 ── guardrail C ──▶ Group 2 ──┬── guardrail D ──┬──▶ done
//	           └── guardrail B ──┘                                         └── guardrail E ──┘
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
