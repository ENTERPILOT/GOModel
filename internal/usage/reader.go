package usage

import (
	"context"
	"time"
)

// UsageQueryParams specifies the query parameters for usage data retrieval.
type UsageQueryParams struct {
	StartDate time.Time // Inclusive start (day precision)
	EndDate   time.Time // Inclusive end (day precision)
	Interval  string    // "daily", "weekly", "monthly", "yearly"
}

// UsageSummary holds aggregated usage statistics over a time period.
type UsageSummary struct {
	TotalRequests  int      `json:"total_requests"`
	TotalInput     int64    `json:"total_input_tokens"`
	TotalOutput    int64    `json:"total_output_tokens"`
	TotalTokens    int64    `json:"total_tokens"`
	TotalInputCost *float64 `json:"total_input_cost"`
	TotalOutputCost *float64 `json:"total_output_cost"`
	TotalCost      *float64 `json:"total_cost"`
}

// ModelUsage holds per-model token usage aggregates.
type ModelUsage struct {
	Model        string `json:"model"`
	Provider     string `json:"provider"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
}

// DailyUsage holds usage statistics for a single period.
// Date holds the period label: YYYY-MM-DD for daily, YYYY-Www for weekly,
// YYYY-MM for monthly, or YYYY for yearly intervals.
type DailyUsage struct {
	Date         string `json:"date"`
	Requests     int    `json:"requests"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	TotalTokens  int64  `json:"total_tokens"`
}

// UsageReader provides read access to usage data for the admin API.
type UsageReader interface {
	// GetSummary returns aggregated usage statistics for the given date range.
	// If both StartDate and EndDate are zero, returns all-time statistics.
	GetSummary(ctx context.Context, params UsageQueryParams) (*UsageSummary, error)

	// GetDailyUsage returns usage statistics grouped by the specified interval.
	// If both StartDate and EndDate are zero, returns all available data.
	GetDailyUsage(ctx context.Context, params UsageQueryParams) ([]DailyUsage, error)

	// GetUsageByModel returns per-model token usage aggregates for the given date range.
	GetUsageByModel(ctx context.Context, params UsageQueryParams) ([]ModelUsage, error)
}
