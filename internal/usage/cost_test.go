package usage

import (
	"math"
	"testing"

	"gomodel/internal/core"
)

func ptr(f float64) *float64 { return &f }

func TestCalculateGranularCost_NilPricing(t *testing.T) {
	result := CalculateGranularCost(100, 50, nil, "openai", nil)
	if result.InputCost != nil || result.OutputCost != nil || result.TotalCost != nil {
		t.Fatal("expected nil costs for nil pricing")
	}
	if result.Caveat != "" {
		t.Fatalf("expected empty caveat, got %q", result.Caveat)
	}
}

func TestCalculateGranularCost_BaseOnly(t *testing.T) {
	pricing := &core.ModelPricing{
		InputPerMtok:  ptr(3.0),
		OutputPerMtok: ptr(15.0),
	}
	result := CalculateGranularCost(1_000_000, 500_000, nil, "openai", pricing)

	assertCostNear(t, "InputCost", result.InputCost, 3.0)
	assertCostNear(t, "OutputCost", result.OutputCost, 7.5)
	assertCostNear(t, "TotalCost", result.TotalCost, 10.5)
	if result.Caveat != "" {
		t.Fatalf("expected empty caveat, got %q", result.Caveat)
	}
}

func TestCalculateGranularCost_OpenAI_CachedAndReasoning(t *testing.T) {
	pricing := &core.ModelPricing{
		InputPerMtok:           ptr(2.50),
		OutputPerMtok:          ptr(10.0),
		CachedInputPerMtok:     ptr(1.25),
		ReasoningOutputPerMtok: ptr(15.0),
	}
	rawData := map[string]any{
		"cached_tokens":    200_000,
		"reasoning_tokens": 100_000,
	}
	result := CalculateGranularCost(500_000, 300_000, rawData, "openai", pricing)

	// Input: 500k * 2.50/1M + 200k * 1.25/1M = 1.25 + 0.25 = 1.50
	assertCostNear(t, "InputCost", result.InputCost, 1.50)
	// Output: 300k * 10.0/1M + 100k * 15.0/1M = 3.0 + 1.5 = 4.5
	assertCostNear(t, "OutputCost", result.OutputCost, 4.5)
	assertCostNear(t, "TotalCost", result.TotalCost, 6.0)
	if result.Caveat != "" {
		t.Fatalf("expected empty caveat, got %q", result.Caveat)
	}
}

func TestCalculateGranularCost_OpenAI_AudioTokens(t *testing.T) {
	pricing := &core.ModelPricing{
		InputPerMtok:      ptr(2.50),
		OutputPerMtok:     ptr(10.0),
		AudioInputPerMtok: ptr(100.0),
		AudioOutputPerMtok: ptr(200.0),
	}
	rawData := map[string]any{
		"prompt_audio_tokens":     50_000,
		"completion_audio_tokens": 30_000,
	}
	result := CalculateGranularCost(100_000, 80_000, rawData, "openai", pricing)

	// Input: 100k * 2.50/1M + 50k * 100/1M = 0.25 + 5.0 = 5.25
	assertCostNear(t, "InputCost", result.InputCost, 5.25)
	// Output: 80k * 10/1M + 30k * 200/1M = 0.80 + 6.0 = 6.80
	assertCostNear(t, "OutputCost", result.OutputCost, 6.80)
}

func TestCalculateGranularCost_Anthropic_CacheTokens(t *testing.T) {
	pricing := &core.ModelPricing{
		InputPerMtok:       ptr(3.0),
		OutputPerMtok:      ptr(15.0),
		CachedInputPerMtok: ptr(0.30),
		CacheWritePerMtok:  ptr(3.75),
	}
	rawData := map[string]any{
		"cache_read_input_tokens":     int64(100_000),
		"cache_creation_input_tokens": 50_000,
	}
	result := CalculateGranularCost(200_000, 100_000, rawData, "anthropic", pricing)

	// Input: 200k * 3.0/1M + 100k * 0.30/1M + 50k * 3.75/1M = 0.60 + 0.03 + 0.1875 = 0.8175
	assertCostNear(t, "InputCost", result.InputCost, 0.8175)
	// Output: 100k * 15.0/1M = 1.5
	assertCostNear(t, "OutputCost", result.OutputCost, 1.5)
}

func TestCalculateGranularCost_Gemini_ThoughtTokens(t *testing.T) {
	pricing := &core.ModelPricing{
		InputPerMtok:           ptr(1.25),
		OutputPerMtok:          ptr(5.0),
		CachedInputPerMtok:     ptr(0.3125),
		ReasoningOutputPerMtok: ptr(10.0),
	}
	rawData := map[string]any{
		"cached_tokens": 50_000,
		"thought_tokens": int(75_000),
	}
	result := CalculateGranularCost(100_000, 200_000, rawData, "gemini", pricing)

	// Input: 100k * 1.25/1M + 50k * 0.3125/1M = 0.125 + 0.015625 = 0.140625
	assertCostNear(t, "InputCost", result.InputCost, 0.140625)
	// Output: 200k * 5.0/1M + 75k * 10.0/1M = 1.0 + 0.75 = 1.75
	assertCostNear(t, "OutputCost", result.OutputCost, 1.75)
}

func TestCalculateGranularCost_XAI_ImageTokens(t *testing.T) {
	pricing := &core.ModelPricing{
		InputPerMtok:  ptr(2.0),
		OutputPerMtok: ptr(10.0),
		InputPerImage: ptr(0.05), // $0.05 per image
	}
	rawData := map[string]any{
		"image_tokens": 3,
	}
	result := CalculateGranularCost(100_000, 50_000, rawData, "xai", pricing)

	// Input: 100k * 2.0/1M + 3 * 0.05 = 0.20 + 0.15 = 0.35
	assertCostNear(t, "InputCost", result.InputCost, 0.35)
}

func TestCalculateGranularCost_MissingPricingCaveat(t *testing.T) {
	pricing := &core.ModelPricing{
		InputPerMtok:  ptr(2.50),
		OutputPerMtok: ptr(10.0),
		// CachedInputPerMtok is nil
	}
	rawData := map[string]any{
		"cached_tokens": 100_000,
	}
	result := CalculateGranularCost(500_000, 300_000, rawData, "openai", pricing)

	if result.Caveat == "" {
		t.Fatal("expected non-empty caveat for missing pricing")
	}
	if result.Caveat != "no pricing for cached_tokens" {
		t.Fatalf("unexpected caveat: %q", result.Caveat)
	}
}

func TestCalculateGranularCost_UnmappedTokenField(t *testing.T) {
	pricing := &core.ModelPricing{
		InputPerMtok:  ptr(2.50),
		OutputPerMtok: ptr(10.0),
	}
	rawData := map[string]any{
		"some_new_tokens": 100,
	}
	result := CalculateGranularCost(100_000, 50_000, rawData, "openai", pricing)

	if result.Caveat == "" {
		t.Fatal("expected caveat for unmapped token field")
	}
	if result.Caveat != "unmapped token field: some_new_tokens" {
		t.Fatalf("unexpected caveat: %q", result.Caveat)
	}
}

func TestCalculateGranularCost_PerRequestFee(t *testing.T) {
	pricing := &core.ModelPricing{
		InputPerMtok:  ptr(0.0),
		OutputPerMtok: ptr(0.0),
		PerRequest:    ptr(0.01),
	}
	result := CalculateGranularCost(100, 50, nil, "openai", pricing)

	assertCostNear(t, "OutputCost", result.OutputCost, 0.01)
	assertCostNear(t, "TotalCost", result.TotalCost, 0.01)
}

func TestCalculateGranularCost_UnknownProvider(t *testing.T) {
	pricing := &core.ModelPricing{
		InputPerMtok:  ptr(1.0),
		OutputPerMtok: ptr(2.0),
	}
	rawData := map[string]any{
		"custom_tokens": 100,
	}
	result := CalculateGranularCost(1_000_000, 500_000, rawData, "unknown_provider", pricing)

	// Base costs should still work
	assertCostNear(t, "InputCost", result.InputCost, 1.0)
	assertCostNear(t, "OutputCost", result.OutputCost, 1.0)
	// Unmapped token field should produce caveat
	if result.Caveat != "unmapped token field: custom_tokens" {
		t.Fatalf("unexpected caveat: %q", result.Caveat)
	}
}

func TestCalculateGranularCost_ZeroTokenRawData(t *testing.T) {
	pricing := &core.ModelPricing{
		InputPerMtok:  ptr(2.50),
		OutputPerMtok: ptr(10.0),
	}
	rawData := map[string]any{
		"cached_tokens": 0,
	}
	// Zero-value token fields should not produce caveats
	result := CalculateGranularCost(100_000, 50_000, rawData, "openai", pricing)
	if result.Caveat != "" {
		t.Fatalf("expected no caveat for zero token count, got %q", result.Caveat)
	}
}

func TestCalculateGranularCost_NonTokenField(t *testing.T) {
	pricing := &core.ModelPricing{
		InputPerMtok:  ptr(2.50),
		OutputPerMtok: ptr(10.0),
	}
	rawData := map[string]any{
		"some_flag": true,
	}
	// Non-token fields should not produce caveats
	result := CalculateGranularCost(100_000, 50_000, rawData, "openai", pricing)
	if result.Caveat != "" {
		t.Fatalf("expected no caveat for non-token field, got %q", result.Caveat)
	}
}

func TestExtractInt(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		key      string
		expected int
	}{
		{"float64", map[string]any{"k": float64(42)}, "k", 42},
		{"int", map[string]any{"k": 42}, "k", 42},
		{"int64", map[string]any{"k": int64(42)}, "k", 42},
		{"string", map[string]any{"k": "42"}, "k", 0},
		{"missing", map[string]any{}, "k", 0},
		{"nil map", nil, "k", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractInt(tt.data, tt.key)
			if got != tt.expected {
				t.Fatalf("extractInt(%v, %q) = %d, want %d", tt.data, tt.key, got, tt.expected)
			}
		})
	}
}

func TestIsTokenField(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"cached_tokens", true},
		{"reasoning_tokens", true},
		{"prompt_token_count", true},
		{"some_flag", false},
		{"model", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := isTokenField(tt.key); got != tt.expected {
				t.Fatalf("isTokenField(%q) = %v, want %v", tt.key, got, tt.expected)
			}
		})
	}
}

func assertCostNear(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s is nil, want %f", name, want)
	}
	if math.Abs(*got-want) > 1e-9 {
		t.Fatalf("%s = %f, want %f", name, *got, want)
	}
}
