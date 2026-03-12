package core

import "context"

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// RequestIDKey is the context key for the request ID.
	requestIDKey contextKey = "request-id"
	// requestSnapshotKey stores the immutable transport snapshot for the request.
	requestSnapshotKey contextKey = "request-snapshot"
	// requestSemanticsKey stores the best-effort semantic extraction for the request.
	requestSemanticsKey contextKey = "request-semantics"
)

// WithRequestID returns a new context with the request ID attached.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID retrieves the request ID from the context.
// Returns empty string if not found.
func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(requestIDKey); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// WithRequestSnapshot returns a new context with the request snapshot attached.
func WithRequestSnapshot(ctx context.Context, snapshot *RequestSnapshot) context.Context {
	return context.WithValue(ctx, requestSnapshotKey, snapshot)
}

// GetRequestSnapshot retrieves the request snapshot from the context.
func GetRequestSnapshot(ctx context.Context) *RequestSnapshot {
	if v := ctx.Value(requestSnapshotKey); v != nil {
		if snapshot, ok := v.(*RequestSnapshot); ok {
			return snapshot
		}
	}
	return nil
}

// WithRequestSemantics returns a new context with the request semantics attached.
func WithRequestSemantics(ctx context.Context, semantics *RequestSemantics) context.Context {
	return context.WithValue(ctx, requestSemanticsKey, semantics)
}

// GetRequestSemantics retrieves the request semantics from the context.
func GetRequestSemantics(ctx context.Context) *RequestSemantics {
	if v := ctx.Value(requestSemanticsKey); v != nil {
		if semantics, ok := v.(*RequestSemantics); ok {
			return semantics
		}
	}
	return nil
}
