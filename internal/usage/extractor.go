package usage

import (
	"time"

	"github.com/google/uuid"

	"gomodel/internal/core"
)

// ExtractFromChatResponse extracts usage data from a ChatResponse.
// It normalizes the usage data into a UsageEntry and preserves raw extended data.
func ExtractFromChatResponse(resp *core.ChatResponse, requestID, provider, endpoint string) *UsageEntry {
	if resp == nil {
		return nil
	}

	entry := &UsageEntry{
		ID:           uuid.New().String(),
		RequestID:    requestID,
		ProviderID:   resp.ID,
		Timestamp:    time.Now().UTC(),
		Model:        resp.Model,
		Provider:     provider,
		Endpoint:     endpoint,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
		TotalTokens:  resp.Usage.TotalTokens,
	}

	// Preserve raw extended usage data if available (defensive copy to avoid races)
	if len(resp.Usage.RawUsage) > 0 {
		entry.RawData = cloneRawData(resp.Usage.RawUsage)
	}

	return entry
}

// cloneRawData creates a shallow copy of the raw data map to prevent races
// when the original map might be mutated after the entry is enqueued.
func cloneRawData(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// ExtractFromResponsesResponse extracts usage data from a ResponsesResponse.
// It normalizes the usage data into a UsageEntry and preserves raw extended data.
func ExtractFromResponsesResponse(resp *core.ResponsesResponse, requestID, provider, endpoint string) *UsageEntry {
	if resp == nil {
		return nil
	}

	entry := &UsageEntry{
		ID:         uuid.New().String(),
		RequestID:  requestID,
		ProviderID: resp.ID,
		Timestamp:  time.Now().UTC(),
		Model:      resp.Model,
		Provider:   provider,
		Endpoint:   endpoint,
	}

	// Extract usage if available
	if resp.Usage != nil {
		entry.InputTokens = resp.Usage.InputTokens
		entry.OutputTokens = resp.Usage.OutputTokens
		entry.TotalTokens = resp.Usage.TotalTokens

		// Preserve raw extended usage data if available (defensive copy to avoid races)
		if len(resp.Usage.RawUsage) > 0 {
			entry.RawData = cloneRawData(resp.Usage.RawUsage)
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

	// Defensive copy to avoid races when original map might be mutated
	if len(rawData) > 0 {
		entry.RawData = cloneRawData(rawData)
	}

	return entry
}
