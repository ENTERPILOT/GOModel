package core

import "encoding/json"

// BatchRequest is OpenAI-compatible for core fields and extends with inline requests.
//
// OpenAI-compatible fields:
//   - input_file_id
//   - endpoint
//   - completion_window
//   - metadata
//
// Gateway extension:
//   - requests (inline payloads for immediate execution without files API)
type BatchRequest struct {
	InputFileID      string             `json:"input_file_id,omitempty"`
	Endpoint         string             `json:"endpoint,omitempty"`
	CompletionWindow string             `json:"completion_window,omitempty"`
	Metadata         map[string]string  `json:"metadata,omitempty"`
	Requests         []BatchRequestItem `json:"requests,omitempty"`
}

// BatchRequestItem represents one sub-request in an inline batch.
type BatchRequestItem struct {
	CustomID string          `json:"custom_id,omitempty"`
	Method   string          `json:"method,omitempty"`
	URL      string          `json:"url"`
	Body     json.RawMessage `json:"body"`
}

// BatchResponse uses OpenAI-compatible batch fields and adds inline execution data.
type BatchResponse struct {
	ID               string             `json:"id"`
	Object           string             `json:"object"`
	Endpoint         string             `json:"endpoint"`
	InputFileID      string             `json:"input_file_id,omitempty"`
	CompletionWindow string             `json:"completion_window,omitempty"`
	Status           string             `json:"status"`
	CreatedAt        int64              `json:"created_at"`
	CompletedAt      *int64             `json:"completed_at,omitempty"`
	RequestCounts    BatchRequestCounts `json:"request_counts"`
	Metadata         map[string]string  `json:"metadata,omitempty"`

	// Gateway extension: inline execution details.
	Usage   BatchUsageSummary `json:"usage"`
	Results []BatchResultItem `json:"results,omitempty"`
}

// BatchRequestCounts is OpenAI-compatible aggregate batch status.
type BatchRequestCounts struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// BatchResultItem represents one sub-response in a batch.
type BatchResultItem struct {
	Index      int         `json:"index"`
	CustomID   string      `json:"custom_id,omitempty"`
	URL        string      `json:"url"`
	StatusCode int         `json:"status_code"`
	Model      string      `json:"model,omitempty"`
	Provider   string      `json:"provider,omitempty"`
	Response   any         `json:"response,omitempty"`
	Error      *BatchError `json:"error,omitempty"`
}

// BatchError represents a normalized error for a failed batch item.
type BatchError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// BatchUsageSummary aggregates usage and cost for successful batch items.
type BatchUsageSummary struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`

	InputCost  *float64 `json:"input_cost,omitempty"`
	OutputCost *float64 `json:"output_cost,omitempty"`
	TotalCost  *float64 `json:"total_cost,omitempty"`
}
