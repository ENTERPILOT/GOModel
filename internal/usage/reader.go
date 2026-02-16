package usage

import "context"

// UsageSummary holds aggregated usage statistics over a time period.
type UsageSummary struct {
	TotalRequests int   `json:"total_requests"`
	TotalInput    int64 `json:"total_input_tokens"`
	TotalOutput   int64 `json:"total_output_tokens"`
	TotalTokens   int64 `json:"total_tokens"`
}

// DailyUsage holds usage statistics for a single day.
type DailyUsage struct {
	Date         string `json:"date"` // YYYY-MM-DD
	Requests     int    `json:"requests"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	TotalTokens  int64  `json:"total_tokens"`
}

// UsageReader provides read access to usage data for the admin API.
type UsageReader interface {
	// GetSummary returns aggregated usage statistics for the last N days.
	// If days <= 0, returns all-time statistics.
	GetSummary(ctx context.Context, days int) (*UsageSummary, error)

	// GetDailyUsage returns daily usage statistics for the last N days.
	// If days <= 0, returns all available daily data.
	GetDailyUsage(ctx context.Context, days int) ([]DailyUsage, error)
}
