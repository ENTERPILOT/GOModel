package auditlog

// Buffer and capture limits for audit logging.
const (
	// MaxBodyCapture is the maximum size of request/response bodies to capture (1MB).
	// Prevents memory exhaustion from large payloads.
	MaxBodyCapture = 1024 * 1024

	// MaxContentCapture is the maximum size of accumulated streaming content (1MB).
	// Used by StreamLogWrapper to limit reconstructed response body size.
	MaxContentCapture = 1024 * 1024

	// SSEBufferSize is the rolling buffer size for extracting usage from SSE streams.
	// Must be large enough to capture the final usage event containing token counts.
	SSEBufferSize = 8192

	// BatchFlushThreshold is the number of entries that triggers an immediate flush.
	// When the batch reaches this size, it's written to storage without waiting for the timer.
	BatchFlushThreshold = 100

	// APIKeyHashPrefixLength is the number of hex characters from SHA256 hash.
	// 16 hex chars = 64 bits of entropy for identification without exposure.
	APIKeyHashPrefixLength = 16
)

// Context keys for storing audit log data in request context.
type contextKey string

const (
	// LogEntryKey is the context key for storing the log entry.
	LogEntryKey contextKey = "auditlog_entry"

	// LogEntryStreamingKey is the context key for marking a request as streaming.
	// When true, the middleware skips logging (StreamLogWrapper handles it instead).
	LogEntryStreamingKey contextKey = "auditlog_entry_streaming"
)
