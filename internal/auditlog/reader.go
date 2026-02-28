package auditlog

import (
	"context"
	"time"
)

// QueryParams specifies the date range for audit log retrieval.
type QueryParams struct {
	StartDate time.Time // Inclusive start (day precision)
	EndDate   time.Time // Inclusive end (day precision)
}

// LogQueryParams specifies query parameters for paginated audit log retrieval.
type LogQueryParams struct {
	QueryParams
	Model      string
	Provider   string
	Method     string
	Path       string
	ErrorType  string
	Search     string
	StatusCode *int
	Stream     *bool
	Limit      int
	Offset     int
}

// LogListResult holds a paginated list of audit log entries.
type LogListResult struct {
	Entries []LogEntry `json:"entries"`
	Total   int        `json:"total"`
	Limit   int        `json:"limit"`
	Offset  int        `json:"offset"`
}

// Reader provides read access to audit log data for the admin API.
type Reader interface {
	// GetLogs returns a paginated list of audit log entries with optional filtering.
	GetLogs(ctx context.Context, params LogQueryParams) (*LogListResult, error)
}
