package app

import (
	"testing"

	"gomodel/config"
	"gomodel/internal/admin"
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

func TestDashboardRuntimeConfig_ExposesFallbackMode(t *testing.T) {
	cfg := &config.Config{
		Fallback: config.FallbackConfig{
			DefaultMode: config.FallbackModeManual,
		},
	}

	values := dashboardRuntimeConfig(cfg)
	if got := values[admin.DashboardConfigFeatureFallbackMode]; got != string(config.FallbackModeManual) {
		t.Fatalf("dashboardRuntimeConfig()[%q] = %q, want %q", admin.DashboardConfigFeatureFallbackMode, got, config.FallbackModeManual)
	}
}
