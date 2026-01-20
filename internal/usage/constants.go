package usage

// Buffer and batch limits for usage tracking.
const (
	// BatchFlushThreshold is the number of entries that triggers an immediate flush.
	// When the batch reaches this size, it's written to storage without waiting for the timer.
	BatchFlushThreshold = 100

	// SSEBufferSize is the rolling buffer size for extracting usage from SSE streams.
	// Must be large enough to capture the final usage event containing token counts.
	SSEBufferSize = 8192
)

// Context keys for storing usage data in request context.
type contextKey string

const (
	// UsageEntryKey is the context key for storing the usage entry.
	UsageEntryKey contextKey = "usage_entry"

	// UsageEntryStreamingKey is the context key for marking a request as streaming.
	// When true, the middleware skips logging (StreamUsageWrapper handles it instead).
	UsageEntryStreamingKey contextKey = "usage_entry_streaming"
)
