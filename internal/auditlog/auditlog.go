// Package auditlog provides audit logging for the AI gateway.
// It captures request/response metadata and stores it in configurable backends.
package auditlog

import (
	"context"
	"strings"
	"time"
)

// LogStore defines the interface for audit log storage backends.
// Implementations must be safe for concurrent use.
type LogStore interface {
	// WriteBatch writes multiple log entries to storage.
	// This is called by the Logger when flushing buffered entries.
	WriteBatch(ctx context.Context, entries []*LogEntry) error

	// Flush forces any pending writes to complete.
	// Called during graceful shutdown.
	Flush(ctx context.Context) error

	// Close releases resources and flushes pending writes.
	Close() error
}

// LogEntry represents a single audit log entry.
// Core fields are indexed for efficient queries.
type LogEntry struct {
	// ID is a unique identifier for this log entry (UUID)
	ID string `json:"id" bson:"_id"`

	// Timestamp is when the request started
	Timestamp time.Time `json:"timestamp" bson:"timestamp"`

	// DurationNs is the request duration in nanoseconds
	DurationNs int64 `json:"duration_ns" bson:"duration_ns"`

	// Core fields (indexed for queries)
	Model      string `json:"model" bson:"model"`
	Provider   string `json:"provider" bson:"provider"`
	StatusCode int    `json:"status_code" bson:"status_code"`

	// Data contains flexible request/response information as JSON
	Data *LogData `json:"data" bson:"data"`
}

// LogData contains flexible request/response information.
// Fields are omitted when empty to save storage space.
type LogData struct {
	// Identity - "Who"
	RequestID  string `json:"request_id,omitempty" bson:"request_id,omitempty"`
	ClientIP   string `json:"client_ip,omitempty" bson:"client_ip,omitempty"`
	UserAgent  string `json:"user_agent,omitempty" bson:"user_agent,omitempty"`
	APIKeyHash string `json:"api_key_hash,omitempty" bson:"api_key_hash,omitempty"`

	// Request - "What" (input)
	Method      string   `json:"method,omitempty" bson:"method,omitempty"`
	Path        string   `json:"path,omitempty" bson:"path,omitempty"`
	Stream      bool     `json:"stream,omitempty" bson:"stream,omitempty"`
	Temperature *float64 `json:"temperature,omitempty" bson:"temperature,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty" bson:"max_tokens,omitempty"`

	// Response - "What" (output)
	PromptTokens     int    `json:"prompt_tokens,omitempty" bson:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty" bson:"completion_tokens,omitempty"`
	TotalTokens      int    `json:"total_tokens,omitempty" bson:"total_tokens,omitempty"`
	ErrorType        string `json:"error_type,omitempty" bson:"error_type,omitempty"`
	ErrorMessage     string `json:"error_message,omitempty" bson:"error_message,omitempty"`

	// Optional headers (when LOGGING_LOG_HEADERS=true)
	// Sensitive headers are auto-redacted
	RequestHeaders  map[string]string `json:"request_headers,omitempty" bson:"request_headers,omitempty"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty" bson:"response_headers,omitempty"`

	// Optional bodies (when LOGGING_LOG_BODIES=true)
	// Stored as interface{} so MongoDB serializes as native BSON documents (queryable/readable)
	// instead of BSON Binary (base64 in Compass)
	RequestBody  interface{} `json:"request_body,omitempty" bson:"request_body,omitempty"`
	ResponseBody interface{} `json:"response_body,omitempty" bson:"response_body,omitempty"`
}

// RedactedHeaders contains headers that should be automatically redacted.
// Values are replaced with "[REDACTED]" to prevent leaking secrets.
var RedactedHeaders = []string{
	"authorization",
	"x-api-key",
	"cookie",
	"set-cookie",
	"x-auth-token",
	"x-access-token",
	"proxy-authorization",
	"x-gomodel-key",
}

// RedactHeaders redacts sensitive headers from a header map.
// The original map is not modified; a new map is returned.
func RedactHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return nil
	}

	result := make(map[string]string, len(headers))
	for key, value := range headers {
		keyLower := strings.ToLower(key)
		redacted := false
		for _, redactKey := range RedactedHeaders {
			if keyLower == redactKey {
				result[key] = "[REDACTED]"
				redacted = true
				break
			}
		}
		if !redacted {
			result[key] = value
		}
	}
	return result
}

// Config holds audit logging configuration
type Config struct {
	// Enabled controls whether audit logging is active
	Enabled bool

	// LogBodies enables logging of full request/response bodies
	LogBodies bool

	// LogHeaders enables logging of request/response headers
	LogHeaders bool

	// BufferSize is the number of log entries to buffer before flushing
	BufferSize int

	// FlushInterval is how often to flush buffered logs
	FlushInterval time.Duration

	// RetentionDays is how long to keep logs (0 = forever)
	RetentionDays int

	// OnlyModelInteractions limits logging to AI model endpoints only
	// When true, only /v1/chat/completions, /v1/responses, /v1/models are logged
	OnlyModelInteractions bool
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() Config {
	return Config{
		Enabled:               false,
		LogBodies:             false,
		LogHeaders:            false,
		BufferSize:            1000,
		FlushInterval:         5 * time.Second,
		RetentionDays:         30,
		OnlyModelInteractions: true,
	}
}
