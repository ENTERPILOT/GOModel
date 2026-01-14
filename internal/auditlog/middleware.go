package auditlog

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// contextKey is a type for context keys to avoid collisions
type contextKey string

const (
	// LogEntryKey is the context key for storing the log entry
	LogEntryKey contextKey = "auditlog_entry"
)

// Middleware creates an Echo middleware for audit logging.
// It captures request metadata at the start and response metadata at the end,
// then writes the log entry asynchronously.
func Middleware(logger LoggerInterface) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip if logging is disabled
			if logger == nil || !logger.Config().Enabled {
				return next(c)
			}

			cfg := logger.Config()
			start := time.Now()
			req := c.Request()

			// Generate request ID if not present
			requestID := req.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.NewString()
			}

			// Create initial log entry
			entry := &LogEntry{
				ID:        uuid.NewString(),
				Timestamp: start,
				Data: &LogData{
					RequestID: requestID,
					ClientIP:  c.RealIP(),
					UserAgent: req.UserAgent(),
					Method:    req.Method,
					Path:      req.URL.Path,
				},
			}

			// Hash API key if present (for identification without exposing the key)
			if authHeader := req.Header.Get("Authorization"); authHeader != "" {
				entry.Data.APIKeyHash = hashAPIKey(authHeader)
			}

			// Log request headers if enabled
			if cfg.LogHeaders {
				entry.Data.RequestHeaders = extractHeaders(req.Header)
			}

			// Capture request body if enabled
			if cfg.LogBodies && req.Body != nil && req.ContentLength > 0 {
				bodyBytes, err := io.ReadAll(req.Body)
				if err == nil {
					entry.Data.RequestBody = bodyBytes
					// Restore the body for the handler
					req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				}
			}

			// Store entry in context for potential enrichment by handlers
			c.Set(string(LogEntryKey), entry)

			// Create response body capture if logging bodies
			var responseCapture *responseBodyCapture
			if cfg.LogBodies {
				responseCapture = &responseBodyCapture{
					ResponseWriter: c.Response().Writer,
					body:           &bytes.Buffer{},
				}
				c.Response().Writer = responseCapture
			}

			// Execute the handler
			err := next(c)

			// Calculate duration
			entry.DurationNs = time.Since(start).Nanoseconds()

			// Capture response metadata
			entry.StatusCode = c.Response().Status

			// Log response headers if enabled
			if cfg.LogHeaders {
				entry.Data.ResponseHeaders = extractEchoHeaders(c.Response().Header())
			}

			// Capture response body if enabled
			if cfg.LogBodies && responseCapture != nil && responseCapture.body.Len() > 0 {
				entry.Data.ResponseBody = responseCapture.body.Bytes()
			}

			// Write log entry asynchronously
			logger.Write(entry)

			return err
		}
	}
}

// responseBodyCapture wraps http.ResponseWriter to capture the response body.
// It implements http.Flusher and http.Hijacker by delegating to the underlying
// ResponseWriter if it supports those interfaces.
type responseBodyCapture struct {
	http.ResponseWriter
	body *bytes.Buffer
}

func (r *responseBodyCapture) Write(b []byte) (int, error) {
	// Write to the capture buffer (limit to 1MB to avoid memory issues)
	if r.body.Len() < 1024*1024 {
		r.body.Write(b)
	}
	// Write to the original response writer
	return r.ResponseWriter.Write(b)
}

// Flush implements http.Flusher. It delegates to the underlying ResponseWriter
// if it implements http.Flusher, otherwise it's a no-op.
// This is required for SSE streaming to work correctly.
func (r *responseBodyCapture) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker. It delegates to the underlying ResponseWriter
// if it implements http.Hijacker, otherwise it returns an error.
// This is required for WebSocket upgrades to work correctly.
func (r *responseBodyCapture) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := r.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// extractHeaders extracts headers from http.Header, redacting sensitive ones
func extractHeaders(headers map[string][]string) map[string]string {
	result := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return RedactHeaders(result)
}

// extractEchoHeaders extracts headers from echo's header map
func extractEchoHeaders(headers map[string][]string) map[string]string {
	result := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return RedactHeaders(result)
}

// hashAPIKey creates a short hash of the API key for identification
// Returns first 8 characters of SHA256 hash
func hashAPIKey(authHeader string) string {
	// Extract token from "Bearer <token>"
	token := strings.TrimPrefix(authHeader, "Bearer ")
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}

	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])[:8]
}

// EnrichEntry retrieves the log entry from context for enrichment by handlers.
// This allows handlers to add model, provider, and token information.
func EnrichEntry(c echo.Context, model, provider string, usage *Usage) {
	entryVal := c.Get(string(LogEntryKey))
	if entryVal == nil {
		return
	}

	entry, ok := entryVal.(*LogEntry)
	if !ok || entry == nil {
		return
	}

	entry.Model = model
	entry.Provider = provider

	if usage != nil {
		entry.Data.PromptTokens = usage.PromptTokens
		entry.Data.CompletionTokens = usage.CompletionTokens
		entry.Data.TotalTokens = usage.TotalTokens
	}
}

// EnrichEntryWithError adds error information to the log entry.
func EnrichEntryWithError(c echo.Context, errorType, errorMessage string) {
	entryVal := c.Get(string(LogEntryKey))
	if entryVal == nil {
		return
	}

	entry, ok := entryVal.(*LogEntry)
	if !ok || entry == nil || entry.Data == nil {
		return
	}

	entry.Data.ErrorType = errorType
	entry.Data.ErrorMessage = errorMessage
}

// EnrichEntryWithStream marks the log entry as a streaming request.
func EnrichEntryWithStream(c echo.Context, stream bool) {
	entryVal := c.Get(string(LogEntryKey))
	if entryVal == nil {
		return
	}

	entry, ok := entryVal.(*LogEntry)
	if !ok || entry == nil || entry.Data == nil {
		return
	}

	entry.Data.Stream = stream
}

// Usage contains token usage information
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}
