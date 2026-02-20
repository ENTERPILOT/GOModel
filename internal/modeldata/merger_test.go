package modeldata

import (
	"math"
	"testing"

	"gomodel/internal/core"
)

func ptr[T any](v T) *T { return &v }

func TestResolve_NilList(t *testing.T) {
	meta := Resolve(nil, "openai", "gpt-4o")
	if meta != nil {
		t.Error("expected nil for nil list")
	}
}

func TestResolve_NoMatch(t *testing.T) {
	list := &ModelList{
		Models:         map[string]ModelEntry{},
		ProviderModels: map[string]ProviderModelEntry{},
	}
	meta := Resolve(list, "openai", "nonexistent-model")
	if meta != nil {
		t.Error("expected nil for no match")
	}
}

func TestResolve_DirectModelMatch(t *testing.T) {
	list := &ModelList{
		Models: map[string]ModelEntry{
			"gpt-4o": {
				DisplayName:     "GPT-4o",
				Description:     ptr("Flagship model"),
				Family:          ptr("gpt-4o"),
				Mode:            "chat",
				Tags:            []string{"flagship", "multimodal"},
				ContextWindow:   ptr(128000),
				MaxOutputTokens: ptr(16384),
				Capabilities: map[string]bool{
					"function_calling": true,
					"streaming":        true,
					"vision":           true,
				},
				Pricing: &PricingEntry{
					Currency:      "USD",
					InputPerMtok:  ptr(2.50),
					OutputPerMtok: ptr(10.00),
				},
			},
		},
		ProviderModels: map[string]ProviderModelEntry{},
	}

	meta := Resolve(list, "openai", "gpt-4o")
	if meta == nil {
		t.Fatal("expected non-nil metadata")
	}

	if meta.DisplayName != "GPT-4o" {
		t.Errorf("DisplayName = %s, want GPT-4o", meta.DisplayName)
	}
	if meta.Description != "Flagship model" {
		t.Errorf("Description = %s, want 'Flagship model'", meta.Description)
	}
	if meta.Family != "gpt-4o" {
		t.Errorf("Family = %s, want gpt-4o", meta.Family)
	}
	if meta.Mode != "chat" {
		t.Errorf("Mode = %s, want chat", meta.Mode)
	}
	if len(meta.Tags) != 2 {
		t.Errorf("Tags len = %d, want 2", len(meta.Tags))
	}
	if *meta.ContextWindow != 128000 {
		t.Errorf("ContextWindow = %d, want 128000", *meta.ContextWindow)
	}
	if *meta.MaxOutputTokens != 16384 {
		t.Errorf("MaxOutputTokens = %d, want 16384", *meta.MaxOutputTokens)
	}
	if !meta.Capabilities["function_calling"] {
		t.Error("expected function_calling capability")
	}
	if meta.Pricing == nil {
		t.Fatal("expected non-nil pricing")
	}
	if meta.Pricing.Currency != "USD" {
		t.Errorf("Currency = %s, want USD", meta.Pricing.Currency)
	}
	if *meta.Pricing.InputPerMtok != 2.50 {
		t.Errorf("InputPerMtok = %f, want 2.50", *meta.Pricing.InputPerMtok)
	}
	if *meta.Pricing.OutputPerMtok != 10.00 {
		t.Errorf("OutputPerMtok = %f, want 10.00", *meta.Pricing.OutputPerMtok)
	}
}

func TestResolve_ProviderModelOverride(t *testing.T) {
	list := &ModelList{
		Models: map[string]ModelEntry{
			"gpt-4o": {
				DisplayName:     "GPT-4o",
				Mode:            "chat",
				ContextWindow:   ptr(128000),
				MaxOutputTokens: ptr(16384),
				Pricing: &PricingEntry{
					Currency:      "USD",
					InputPerMtok:  ptr(2.50),
					OutputPerMtok: ptr(10.00),
				},
			},
		},
		ProviderModels: map[string]ProviderModelEntry{
			"azure/gpt-4o": {
				ModelRef:      "gpt-4o",
				Enabled:       true,
				ContextWindow: ptr(64000),
				Pricing: &PricingEntry{
					Currency:      "USD",
					InputPerMtok:  ptr(5.00),
					OutputPerMtok: ptr(15.00),
				},
			},
		},
	}

	meta := Resolve(list, "azure", "gpt-4o")
	if meta == nil {
		t.Fatal("expected non-nil metadata")
	}

	// Provider model should override context_window
	if *meta.ContextWindow != 64000 {
		t.Errorf("ContextWindow = %d, want 64000 (override)", *meta.ContextWindow)
	}
	// max_output_tokens should come from base model (not overridden)
	if *meta.MaxOutputTokens != 16384 {
		t.Errorf("MaxOutputTokens = %d, want 16384 (base)", *meta.MaxOutputTokens)
	}
	// Pricing should be overridden
	if *meta.Pricing.InputPerMtok != 5.00 {
		t.Errorf("InputPerMtok = %f, want 5.00 (override)", *meta.Pricing.InputPerMtok)
	}
	if *meta.Pricing.OutputPerMtok != 15.00 {
		t.Errorf("OutputPerMtok = %f, want 15.00 (override)", *meta.Pricing.OutputPerMtok)
	}
	// DisplayName from base model
	if meta.DisplayName != "GPT-4o" {
		t.Errorf("DisplayName = %s, want GPT-4o (base)", meta.DisplayName)
	}
}

func TestResolve_ProviderModelWithoutBaseModel(t *testing.T) {
	list := &ModelList{
		Models: map[string]ModelEntry{},
		ProviderModels: map[string]ProviderModelEntry{
			"custom/my-model": {
				ModelRef:      "nonexistent",
				Enabled:       true,
				ContextWindow: ptr(32000),
				Pricing: &PricingEntry{
					Currency:      "USD",
					InputPerMtok:  ptr(1.00),
					OutputPerMtok: ptr(2.00),
				},
			},
		},
	}

	meta := Resolve(list, "custom", "my-model")
	if meta == nil {
		t.Fatal("expected non-nil metadata even without base model")
	}

	if *meta.ContextWindow != 32000 {
		t.Errorf("ContextWindow = %d, want 32000", *meta.ContextWindow)
	}
	if *meta.Pricing.InputPerMtok != 1.00 {
		t.Errorf("InputPerMtok = %f, want 1.00", *meta.Pricing.InputPerMtok)
	}
}

func TestResolve_NilPricing(t *testing.T) {
	list := &ModelList{
		Models: map[string]ModelEntry{
			"text-moderation": {
				DisplayName: "Text Moderation",
				Mode:        "moderation",
			},
		},
		ProviderModels: map[string]ProviderModelEntry{},
	}

	meta := Resolve(list, "openai", "text-moderation")
	if meta == nil {
		t.Fatal("expected non-nil metadata")
	}
	if meta.Pricing != nil {
		t.Error("expected nil pricing for model without pricing")
	}
}

func TestConvertPricing_Nil(t *testing.T) {
	p := convertPricing(nil)
	if p != nil {
		t.Error("expected nil for nil input")
	}
}

func TestConvertPricing_WithCachedInput(t *testing.T) {
	p := convertPricing(&PricingEntry{
		Currency:           "USD",
		InputPerMtok:       ptr(2.50),
		OutputPerMtok:      ptr(10.00),
		CachedInputPerMtok: ptr(1.25),
	})
	if p == nil {
		t.Fatal("expected non-nil pricing")
	}
	if p.CachedInputPerMtok == nil || *p.CachedInputPerMtok != 1.25 {
		t.Errorf("CachedInputPerMtok = %v, want 1.25", p.CachedInputPerMtok)
	}
}

func TestConvertPricing_AllFields(t *testing.T) {
	p := convertPricing(&PricingEntry{
		Currency:               "USD",
		InputPerMtok:           ptr(2.50),
		OutputPerMtok:          ptr(10.00),
		CachedInputPerMtok:     ptr(1.25),
		ReasoningOutputPerMtok: ptr(60.00),
		PerImage:               ptr(0.04),
		PerSecondInput:         ptr(0.006),
		PerSecondOutput:        ptr(0.012),
		PerCharacterInput:      ptr(0.000015),
		PerRequest:             ptr(0.001),
		PerPage:                ptr(0.005),
	})
	if p == nil {
		t.Fatal("expected non-nil pricing")
	}

	checks := []struct {
		name string
		got  *float64
		want float64
	}{
		{"InputPerMtok", p.InputPerMtok, 2.50},
		{"OutputPerMtok", p.OutputPerMtok, 10.00},
		{"CachedInputPerMtok", p.CachedInputPerMtok, 1.25},
		{"ReasoningOutputPerMtok", p.ReasoningOutputPerMtok, 60.00},
		{"PerImage", p.PerImage, 0.04},
		{"PerSecondInput", p.PerSecondInput, 0.006},
		{"PerSecondOutput", p.PerSecondOutput, 0.012},
		{"PerCharacterInput", p.PerCharacterInput, 0.000015},
		{"PerRequest", p.PerRequest, 0.001},
		{"PerPage", p.PerPage, 0.005},
	}

	for _, c := range checks {
		if c.got == nil {
			t.Errorf("%s: got nil, want %f", c.name, c.want)
		} else if math.Abs(*c.got-c.want) > 1e-9 {
			t.Errorf("%s: got %f, want %f", c.name, *c.got, c.want)
		}
	}

	if p.Currency != "USD" {
		t.Errorf("Currency = %s, want USD", p.Currency)
	}
}

func TestResolve_SetsCategoryFromMode(t *testing.T) {
	list := &ModelList{
		Models: map[string]ModelEntry{
			"gpt-4o": {
				DisplayName: "GPT-4o",
				Mode:        "chat",
			},
			"dall-e-3": {
				DisplayName: "DALL-E 3",
				Mode:        "image_generation",
			},
			"whisper-1": {
				DisplayName: "Whisper",
				Mode:        "audio_transcription",
			},
			"text-moderation": {
				DisplayName: "Moderation",
				Mode:        "moderation",
			},
		},
		ProviderModels: map[string]ProviderModelEntry{},
	}

	tests := []struct {
		modelID  string
		wantCat  core.ModelCategory
	}{
		{"gpt-4o", core.CategoryTextGeneration},
		{"dall-e-3", core.CategoryImage},
		{"whisper-1", core.CategoryAudio},
		{"text-moderation", core.CategoryUtility},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			meta := Resolve(list, "openai", tt.modelID)
			if meta == nil {
				t.Fatal("expected non-nil metadata")
			}
			if meta.Category != tt.wantCat {
				t.Errorf("Category = %q, want %q", meta.Category, tt.wantCat)
			}
		})
	}
}

// Verify Resolve handles the three-layer merge correctly:
// base model fields + provider_model overrides
func TestResolve_ThreeLayerMerge(t *testing.T) {
	list := &ModelList{
		Models: map[string]ModelEntry{
			"claude-sonnet-4-20250514": {
				DisplayName:     "Claude Sonnet 4",
				Description:     ptr("Fast, intelligent model"),
				Family:          ptr("claude-sonnet"),
				Mode:            "chat",
				Tags:            []string{"flagship"},
				ContextWindow:   ptr(200000),
				MaxOutputTokens: ptr(16384),
				Capabilities: map[string]bool{
					"function_calling": true,
					"vision":           true,
				},
				Pricing: &PricingEntry{
					Currency:           "USD",
					InputPerMtok:       ptr(3.00),
					OutputPerMtok:      ptr(15.00),
					CachedInputPerMtok: ptr(0.30),
				},
			},
		},
		ProviderModels: map[string]ProviderModelEntry{
			"bedrock/claude-sonnet-4-20250514": {
				ModelRef:        "claude-sonnet-4-20250514",
				Enabled:         true,
				MaxOutputTokens: ptr(8192),
				Pricing: &PricingEntry{
					Currency:           "USD",
					InputPerMtok:       ptr(3.00),
					OutputPerMtok:      ptr(15.00),
					CachedInputPerMtok: ptr(0.30),
				},
			},
		},
	}

	// Direct provider (anthropic) - should use base model only
	meta := Resolve(list, "anthropic", "claude-sonnet-4-20250514")
	if meta == nil {
		t.Fatal("expected non-nil metadata")
	}
	if *meta.MaxOutputTokens != 16384 {
		t.Errorf("MaxOutputTokens = %d, want 16384 (base)", *meta.MaxOutputTokens)
	}

	// Bedrock - should override max_output_tokens
	meta = Resolve(list, "bedrock", "claude-sonnet-4-20250514")
	if meta == nil {
		t.Fatal("expected non-nil metadata for bedrock")
	}
	if *meta.MaxOutputTokens != 8192 {
		t.Errorf("MaxOutputTokens = %d, want 8192 (bedrock override)", *meta.MaxOutputTokens)
	}
	// DisplayName should still come from base
	if meta.DisplayName != "Claude Sonnet 4" {
		t.Errorf("DisplayName = %s, want 'Claude Sonnet 4'", meta.DisplayName)
	}
}

func TestResolve_ReverseProviderModelIDLookup(t *testing.T) {
	list := &ModelList{
		Models: map[string]ModelEntry{
			"gpt-4o": {
				DisplayName:   "GPT-4o",
				Mode:          "chat",
				ContextWindow: ptr(128000),
				Pricing: &PricingEntry{
					Currency:      "USD",
					InputPerMtok:  ptr(2.50),
					OutputPerMtok: ptr(10.00),
				},
			},
		},
		ProviderModels: map[string]ProviderModelEntry{
			"openai/gpt-4o": {
				ModelRef:        "gpt-4o",
				ProviderModelID: ptr("gpt-4o-2024-08-06"),
				Enabled:         true,
			},
		},
	}
	list.buildReverseIndex()

	// Resolve using the dated response model ID
	meta := Resolve(list, "openai", "gpt-4o-2024-08-06")
	if meta == nil {
		t.Fatal("expected non-nil metadata via reverse lookup")
	}
	if meta.DisplayName != "GPT-4o" {
		t.Errorf("DisplayName = %s, want GPT-4o", meta.DisplayName)
	}
	if meta.Pricing == nil {
		t.Fatal("expected non-nil pricing via reverse lookup")
	}
	if *meta.Pricing.InputPerMtok != 2.50 {
		t.Errorf("InputPerMtok = %f, want 2.50", *meta.Pricing.InputPerMtok)
	}
}

func TestResolve_ReverseIndexNotBuilt(t *testing.T) {
	list := &ModelList{
		Models: map[string]ModelEntry{
			"gpt-4o": {
				DisplayName: "GPT-4o",
				Mode:        "chat",
			},
		},
		ProviderModels: map[string]ProviderModelEntry{
			"openai/gpt-4o": {
				ModelRef:        "gpt-4o",
				ProviderModelID: ptr("gpt-4o-2024-08-06"),
				Enabled:         true,
			},
		},
		// providerModelByActualID is nil (buildReverseIndex not called)
	}

	meta := Resolve(list, "openai", "gpt-4o-2024-08-06")
	if meta != nil {
		t.Error("expected nil when reverse index is not built")
	}
}

func TestResolve_ReverseIndexWithProviderModelOverride(t *testing.T) {
	list := &ModelList{
		Models: map[string]ModelEntry{
			"gpt-4o": {
				DisplayName:   "GPT-4o",
				Mode:          "chat",
				ContextWindow: ptr(128000),
				Pricing: &PricingEntry{
					Currency:      "USD",
					InputPerMtok:  ptr(2.50),
					OutputPerMtok: ptr(10.00),
				},
			},
		},
		ProviderModels: map[string]ProviderModelEntry{
			"openai/gpt-4o": {
				ModelRef:        "gpt-4o",
				ProviderModelID: ptr("gpt-4o-2024-08-06"),
				Enabled:         true,
				Pricing: &PricingEntry{
					Currency:      "USD",
					InputPerMtok:  ptr(3.00),
					OutputPerMtok: ptr(12.00),
				},
			},
		},
	}
	list.buildReverseIndex()

	// Reverse lookup should resolve and apply provider_model pricing override
	meta := Resolve(list, "openai", "gpt-4o-2024-08-06")
	if meta == nil {
		t.Fatal("expected non-nil metadata via reverse lookup")
	}
	if meta.Pricing == nil {
		t.Fatal("expected non-nil pricing")
	}
	// Should use the provider_model override, not the base model pricing
	if *meta.Pricing.InputPerMtok != 3.00 {
		t.Errorf("InputPerMtok = %f, want 3.00 (provider override)", *meta.Pricing.InputPerMtok)
	}
	if *meta.Pricing.OutputPerMtok != 12.00 {
		t.Errorf("OutputPerMtok = %f, want 12.00 (provider override)", *meta.Pricing.OutputPerMtok)
	}
}

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name        string
		input       int
		output      int
		pricing     *core.ModelPricing
		wantInput   *float64
		wantOutput  *float64
		wantTotal   *float64
	}{
		{
			name:    "nil pricing",
			input:   1000,
			output:  500,
			pricing: nil,
		},
		{
			name:   "basic pricing",
			input:  1000000,
			output: 500000,
			pricing: &core.ModelPricing{
				Currency:      "USD",
				InputPerMtok:  ptr(2.50),
				OutputPerMtok: ptr(10.00),
			},
			wantInput:  ptr(2.50),
			wantOutput: ptr(5.00),
			wantTotal:  ptr(7.50),
		},
		{
			name:   "zero tokens",
			input:  0,
			output: 0,
			pricing: &core.ModelPricing{
				Currency:      "USD",
				InputPerMtok:  ptr(2.50),
				OutputPerMtok: ptr(10.00),
			},
			wantInput:  ptr(0.0),
			wantOutput: ptr(0.0),
			wantTotal:  ptr(0.0),
		},
		{
			name:   "partial pricing (input only)",
			input:  1000000,
			output: 500000,
			pricing: &core.ModelPricing{
				Currency:     "USD",
				InputPerMtok: ptr(2.50),
			},
			wantInput: ptr(2.50),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// CalculateCost is in the usage package, but we test the logic inline
			in, out, total := calculateCostHelper(tt.input, tt.output, tt.pricing)

			assertPtrFloat(t, "input", in, tt.wantInput)
			assertPtrFloat(t, "output", out, tt.wantOutput)
			assertPtrFloat(t, "total", total, tt.wantTotal)
		})
	}
}

// calculateCostHelper intentionally mirrors usage.CalculateCost for testing within
// this package. It is duplicated here (rather than imported) to avoid import cycles
// between modeldata and usage. The canonical implementation lives in
// usage.CalculateCost — keep both in sync when changing cost calculation logic.
func calculateCostHelper(inputTokens, outputTokens int, pricing *core.ModelPricing) (input, output, total *float64) {
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

func assertPtrFloat(t *testing.T, name string, got, want *float64) {
	t.Helper()
	if got == nil && want == nil {
		return
	}
	if got == nil {
		t.Errorf("%s: got nil, want %f", name, *want)
		return
	}
	if want == nil {
		t.Errorf("%s: got %f, want nil", name, *got)
		return
	}
	const eps = 1e-9
	if math.Abs(*got-*want) > eps {
		t.Errorf("%s: got %f, want %f", name, *got, *want)
	}
}
