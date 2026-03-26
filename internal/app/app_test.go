package app

import (
	"testing"

	"gomodel/config"
)

func TestRuntimeExecutionFeatureCaps_EnableFallbackFromOverride(t *testing.T) {
	cfg := &config.Config{
		Fallback: config.FallbackConfig{
			DefaultMode: config.FallbackModeOff,
			Overrides: map[string]config.FallbackModelOverride{
				"gpt-4o": {Mode: config.FallbackModeManual},
			},
		},
	}

	caps := runtimeExecutionFeatureCaps(cfg)
	if !caps.Fallback {
		t.Fatal("runtimeExecutionFeatureCaps().Fallback = false, want true")
	}
}

func TestDefaultExecutionPlanInput_SetsFallbackFeature(t *testing.T) {
	cfg := &config.Config{
		Fallback: config.FallbackConfig{
			DefaultMode: config.FallbackModeAuto,
		},
	}

	input := defaultExecutionPlanInput(cfg)
	if input.Payload.Features.Fallback == nil {
		t.Fatal("defaultExecutionPlanInput().Payload.Features.Fallback = nil, want non-nil")
	}
	if !*input.Payload.Features.Fallback {
		t.Fatal("defaultExecutionPlanInput().Payload.Features.Fallback = false, want true")
	}
}
