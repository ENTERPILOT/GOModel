package usage

import (
	"time"

	"github.com/google/uuid"

	"gomodel/internal/core"
)

// CalculateCost computes input, output, and total costs from token counts and pricing.
// Returns nil pointers when pricing info is unavailable.
func CalculateCost(inputTokens, outputTokens int, pricing *core.ModelPricing) (input, output, total *float64) {
	if pricing == nil {
		return nil, nil, nil
	}
	if pricing.InputPerMtok != nil {
		v := float64(inputTokens) * *pricing.InputPerMtok / 1_000_000
		input = &v
	}
	if pricing.OutputPerMtok != nil {
		v := float64(outputTokens) * *pricing.OutputPerMtok / 1_000_000
		output = &v
	}
	if input != nil && output != nil {
		v := *input + *output
		total = &v
	}
	return
}

// ExtractFromChatResponse extracts usage data from a ChatResponse.
// It normalizes the usage data into a UsageEntry and preserves raw extended data.
// If pricing is provided, granular cost fields are calculated.
func ExtractFromChatResponse(resp *core.ChatResponse, requestID, provider, endpoint string, pricing ...*core.ModelPricing) *UsageEntry {
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

	// Calculate granular costs if pricing is provided
	if len(pricing) > 0 && pricing[0] != nil {
		costResult := CalculateGranularCost(entry.InputTokens, entry.OutputTokens, entry.RawData, provider, pricing[0])
		entry.InputCost = costResult.InputCost
		entry.OutputCost = costResult.OutputCost
		entry.TotalCost = costResult.TotalCost
		entry.CostsCalculationCaveat = costResult.Caveat
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
// If pricing is provided, cost fields are calculated.
func ExtractFromResponsesResponse(resp *core.ResponsesResponse, requestID, provider, endpoint string, pricing ...*core.ModelPricing) *UsageEntry {
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

	// Calculate granular costs if pricing is provided
	if len(pricing) > 0 && pricing[0] != nil {
		costResult := CalculateGranularCost(entry.InputTokens, entry.OutputTokens, entry.RawData, provider, pricing[0])
		entry.InputCost = costResult.InputCost
		entry.OutputCost = costResult.OutputCost
		entry.TotalCost = costResult.TotalCost
		entry.CostsCalculationCaveat = costResult.Caveat
	}

	return entry
}

// ExtractFromSSEUsage creates a UsageEntry from SSE-extracted usage data.
// This is used for streaming responses where usage is extracted from the final SSE event.
// If pricing is provided, cost fields are calculated.
func ExtractFromSSEUsage(
	providerID string,
	inputTokens, outputTokens, totalTokens int,
	rawData map[string]any,
	requestID, model, provider, endpoint string,
	pricing ...*core.ModelPricing,
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

	// Calculate granular costs if pricing is provided
	if len(pricing) > 0 && pricing[0] != nil {
		costResult := CalculateGranularCost(entry.InputTokens, entry.OutputTokens, entry.RawData, provider, pricing[0])
		entry.InputCost = costResult.InputCost
		entry.OutputCost = costResult.OutputCost
		entry.TotalCost = costResult.TotalCost
		entry.CostsCalculationCaveat = costResult.Caveat
	}

	return entry
}
