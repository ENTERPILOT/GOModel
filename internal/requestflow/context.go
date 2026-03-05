package requestflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"gomodel/config"
)

type contextKey string

const (
	executionContextKey contextKey = "request-flow-execution"
	selectorContextKey  contextKey = "request-flow-selector"
)

// SelectorContext holds request attributes used for plan matching.
type SelectorContext struct {
	APIKeyHash string
	Team       string
	User       string
}

// ExecutionState keeps mutable per-request flow metadata in context.
type ExecutionState struct {
	mu            sync.Mutex
	execution     *Execution
	retryOverride config.RetryConfig
	start         time.Time
}

// WithExecutionState attaches request flow execution tracking to a context.
func WithExecutionState(ctx context.Context, exec *Execution, retry config.RetryConfig) context.Context {
	state := &ExecutionState{
		execution:     exec,
		retryOverride: retry,
		start:         time.Now(),
	}
	return context.WithValue(ctx, executionContextKey, state)
}

// StateFromContext extracts the execution state from context.
func StateFromContext(ctx context.Context) *ExecutionState {
	if v := ctx.Value(executionContextKey); v != nil {
		if state, ok := v.(*ExecutionState); ok {
			return state
		}
	}
	return nil
}

// WithSelectorContext attaches plan selector metadata to the context.
func WithSelectorContext(ctx context.Context, selector SelectorContext) context.Context {
	return context.WithValue(ctx, selectorContextKey, selector)
}

// SelectorFromContext extracts selector metadata from the context.
func SelectorFromContext(ctx context.Context) SelectorContext {
	if v := ctx.Value(selectorContextKey); v != nil {
		if selector, ok := v.(SelectorContext); ok {
			return selector
		}
	}
	return SelectorContext{}
}

// RetryOverrideFromContext returns the active retry override when present.
func RetryOverrideFromContext(ctx context.Context) (config.RetryConfig, bool) {
	state := StateFromContext(ctx)
	if state == nil {
		return config.RetryConfig{}, false
	}
	return state.retryOverride, true
}

// RecordAttempt increments upstream attempts and tracks the provider.
func RecordAttempt(ctx context.Context, provider string) {
	state := StateFromContext(ctx)
	if state == nil || state.execution == nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.execution.UpstreamAttempts++
	if provider != "" {
		state.execution.Provider = provider
	}
	if state.execution.UpstreamAttempts > 1 {
		state.execution.RetriesMade = state.execution.UpstreamAttempts - 1
	}
}

// RecordGuardrailsApplied stores the guardrails that actually ran.
func RecordGuardrailsApplied(ctx context.Context, names []string) {
	state := StateFromContext(ctx)
	if state == nil || state.execution == nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.execution.GuardrailsApplied = append([]string(nil), names...)
}

// Finish marks the execution outcome and returns a detached copy ready for persistence.
func Finish(ctx context.Context, provider string, status string, err error) *Execution {
	state := StateFromContext(ctx)
	if state == nil || state.execution == nil {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.execution.Duration = time.Since(state.start)
	if provider != "" {
		state.execution.Provider = provider
	}
	state.execution.Status = status
	if err != nil {
		state.execution.Error = err.Error()
	}
	copyExec := *state.execution
	copyExec.PlanIDs = append([]string(nil), state.execution.PlanIDs...)
	copyExec.PlanSources = append([]string(nil), state.execution.PlanSources...)
	copyExec.GuardrailsConfigured = append([]string(nil), state.execution.GuardrailsConfigured...)
	copyExec.GuardrailsApplied = append([]string(nil), state.execution.GuardrailsApplied...)
	return &copyExec
}

// HashAPIKey hashes an Authorization header into a stable identifier.
func HashAPIKey(authHeader string) string {
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])[:12]
}
