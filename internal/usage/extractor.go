package usage

import (
	"time"

	"github.com/google/uuid"

	"gomodel/internal/core"
)

// ExtractFromChatResponse extracts usage data from a ChatResponse.
// It normalizes the usage data into a UsageEntry and preserves raw extended data.
func ExtractFromChatResponse(resp *core.ChatResponse, requestID, endpoint string) *UsageEntry {
	if resp == nil {
		return nil
	}

	entry := &UsageEntry{
		ID:           uuid.New().String(),
		RequestID:    requestID,
		ProviderID:   resp.ID,
		Timestamp:    time.Now().UTC(),
		Model:        resp.Model,
		Provider:     resp.Provider,
		Endpoint:     endpoint,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
		TotalTokens:  resp.Usage.TotalTokens,
	}

	// Preserve raw extended usage data if available
	if len(resp.Usage.RawUsage) > 0 {
		entry.RawData = resp.Usage.RawUsage
	}

	return entry
}

// ExtractFromResponsesResponse extracts usage data from a ResponsesResponse.
// It normalizes the usage data into a UsageEntry and preserves raw extended data.
func ExtractFromResponsesResponse(resp *core.ResponsesResponse, requestID, endpoint string) *UsageEntry {
	if resp == nil {
		return nil
	}

	entry := &UsageEntry{
		ID:         uuid.New().String(),
		RequestID:  requestID,
		ProviderID: resp.ID,
		Timestamp:  time.Now().UTC(),
		Model:      resp.Model,
		Provider:   resp.Provider,
		Endpoint:   endpoint,
	}

	// Extract usage if available
	if resp.Usage != nil {
		entry.InputTokens = resp.Usage.InputTokens
		entry.OutputTokens = resp.Usage.OutputTokens
		entry.TotalTokens = resp.Usage.TotalTokens

		// Preserve raw extended usage data if available
		if len(resp.Usage.RawUsage) > 0 {
			entry.RawData = resp.Usage.RawUsage
		}
	}

	return entry
}

// ExtractFromSSEUsage creates a UsageEntry from SSE-extracted usage data.
// This is used for streaming responses where usage is extracted from the final SSE event.
func ExtractFromSSEUsage(
	providerID string,
	inputTokens, outputTokens, totalTokens int,
	rawData map[string]any,
	requestID, model, provider, endpoint string,
) *UsageEntry {
	entry := &UsageEntry{
		ID:           uuid.New().String(),
		RequestID:    requestID,
		ProviderID:   providerID,
		Timestamp:    time.Now().UTC(),
		Model:        model,
		Provider:     provider,
		Endpoint:     endpoint,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  totalTokens,
	}

	if len(rawData) > 0 {
		entry.RawData = rawData
	}

	return entry
}
