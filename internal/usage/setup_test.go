package usage

import (
	"os"
	"testing"

	"gomodel/internal/core"
)

func TestMain(m *testing.M) {
	// Register cost mappings that were previously hardcoded in cost.go.
	// This mirrors the data that providers supply via their Registration vars.
	RegisterCostMappings(map[string][]core.TokenCostMapping{
		"openai": {
			{RawDataKey: "cached_tokens", PricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, Side: core.CostSideInput, Unit: core.CostUnitPerMtok},
			{RawDataKey: "prompt_cached_tokens", PricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, Side: core.CostSideInput, Unit: core.CostUnitPerMtok},
			{RawDataKey: "reasoning_tokens", PricingField: func(p *core.ModelPricing) *float64 { return p.ReasoningOutputPerMtok }, Side: core.CostSideOutput, Unit: core.CostUnitPerMtok},
			{RawDataKey: "completion_reasoning_tokens", PricingField: func(p *core.ModelPricing) *float64 { return p.ReasoningOutputPerMtok }, Side: core.CostSideOutput, Unit: core.CostUnitPerMtok},
			{RawDataKey: "prompt_audio_tokens", PricingField: func(p *core.ModelPricing) *float64 { return p.AudioInputPerMtok }, Side: core.CostSideInput, Unit: core.CostUnitPerMtok},
			{RawDataKey: "completion_audio_tokens", PricingField: func(p *core.ModelPricing) *float64 { return p.AudioOutputPerMtok }, Side: core.CostSideOutput, Unit: core.CostUnitPerMtok},
		},
		"anthropic": {
			{RawDataKey: "cache_read_input_tokens", PricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, Side: core.CostSideInput, Unit: core.CostUnitPerMtok},
			{RawDataKey: "cache_creation_input_tokens", PricingField: func(p *core.ModelPricing) *float64 { return p.CacheWritePerMtok }, Side: core.CostSideInput, Unit: core.CostUnitPerMtok},
		},
		"gemini": {
			{RawDataKey: "cached_tokens", PricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, Side: core.CostSideInput, Unit: core.CostUnitPerMtok},
			{RawDataKey: "thought_tokens", PricingField: func(p *core.ModelPricing) *float64 { return p.ReasoningOutputPerMtok }, Side: core.CostSideOutput, Unit: core.CostUnitPerMtok},
		},
		"xai": {
			{RawDataKey: "cached_tokens", PricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, Side: core.CostSideInput, Unit: core.CostUnitPerMtok},
			{RawDataKey: "prompt_cached_tokens", PricingField: func(p *core.ModelPricing) *float64 { return p.CachedInputPerMtok }, Side: core.CostSideInput, Unit: core.CostUnitPerMtok},
			{RawDataKey: "reasoning_tokens", PricingField: func(p *core.ModelPricing) *float64 { return p.ReasoningOutputPerMtok }, Side: core.CostSideOutput, Unit: core.CostUnitPerMtok},
			{RawDataKey: "completion_reasoning_tokens", PricingField: func(p *core.ModelPricing) *float64 { return p.ReasoningOutputPerMtok }, Side: core.CostSideOutput, Unit: core.CostUnitPerMtok},
			{RawDataKey: "image_tokens", PricingField: func(p *core.ModelPricing) *float64 { return p.InputPerImage }, Side: core.CostSideInput, Unit: core.CostUnitPerItem},
		},
	}, []string{
		"prompt_text_tokens",
		"prompt_image_tokens",
		"completion_accepted_prediction_tokens",
		"completion_rejected_prediction_tokens",
	})

	os.Exit(m.Run())
}
