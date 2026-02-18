package providers

import (
	"testing"
	"time"

	"gomodel/config"
)

// ptr helpers
func intPtr(v int) *int               { return &v }
func durPtr(v time.Duration) *time.Duration { return &v }
func f64Ptr(v float64) *float64       { return &v }

var globalRetry = config.RetryConfig{
	MaxRetries:     3,
	InitialBackoff: 1 * time.Second,
	MaxBackoff:     30 * time.Second,
	BackoffFactor:  2.0,
	JitterFactor:   0.1,
}

var globalResilience = config.ResilienceConfig{Retry: globalRetry}

// --- buildProviderConfig ---

func TestBuildProviderConfig_InheritsGlobal(t *testing.T) {
	raw := config.RawProviderConfig{Type: "openai", APIKey: "sk-test"}
	got := buildProviderConfig(raw, globalResilience)

	if got.Type != "openai" {
		t.Errorf("Type = %q, want openai", got.Type)
	}
	if got.Resilience.Retry != globalRetry {
		t.Errorf("expected global retry to be inherited\ngot:  %+v\nwant: %+v", got.Resilience.Retry, globalRetry)
	}
}

func TestBuildProviderConfig_NilResilience(t *testing.T) {
	raw := config.RawProviderConfig{Type: "openai", APIKey: "sk", Resilience: nil}
	got := buildProviderConfig(raw, globalResilience)

	if got.Resilience.Retry != globalRetry {
		t.Error("nil Resilience should inherit global")
	}
}

func TestBuildProviderConfig_NilRetry(t *testing.T) {
	raw := config.RawProviderConfig{
		Type:       "openai",
		APIKey:     "sk",
		Resilience: &config.RawResilienceConfig{Retry: nil},
	}
	got := buildProviderConfig(raw, globalResilience)

	if got.Resilience.Retry != globalRetry {
		t.Error("nil Retry should inherit global")
	}
}

func TestBuildProviderConfig_PartialOverride(t *testing.T) {
	raw := config.RawProviderConfig{
		Type:   "anthropic",
		APIKey: "sk-ant",
		Resilience: &config.RawResilienceConfig{
			Retry: &config.RawRetryConfig{
				MaxRetries: intPtr(10),
			},
		},
	}
	got := buildProviderConfig(raw, globalResilience)

	if got.Resilience.Retry.MaxRetries != 10 {
		t.Errorf("MaxRetries = %d, want 10", got.Resilience.Retry.MaxRetries)
	}
	if got.Resilience.Retry.InitialBackoff != globalRetry.InitialBackoff {
		t.Errorf("InitialBackoff should be inherited, got %v", got.Resilience.Retry.InitialBackoff)
	}
	if got.Resilience.Retry.JitterFactor != globalRetry.JitterFactor {
		t.Errorf("JitterFactor should be inherited, got %f", got.Resilience.Retry.JitterFactor)
	}
}

func TestBuildProviderConfig_FullOverride(t *testing.T) {
	raw := config.RawProviderConfig{
		Type:   "gemini",
		APIKey: "sk-gem",
		Resilience: &config.RawResilienceConfig{
			Retry: &config.RawRetryConfig{
				MaxRetries:     intPtr(7),
				InitialBackoff: durPtr(500 * time.Millisecond),
				MaxBackoff:     durPtr(10 * time.Second),
				BackoffFactor:  f64Ptr(1.5),
				JitterFactor:   f64Ptr(0.3),
			},
		},
	}
	got := buildProviderConfig(raw, globalResilience)

	r := got.Resilience.Retry
	if r.MaxRetries != 7 {
		t.Errorf("MaxRetries = %d, want 7", r.MaxRetries)
	}
	if r.InitialBackoff != 500*time.Millisecond {
		t.Errorf("InitialBackoff = %v, want 500ms", r.InitialBackoff)
	}
	if r.MaxBackoff != 10*time.Second {
		t.Errorf("MaxBackoff = %v, want 10s", r.MaxBackoff)
	}
	if r.BackoffFactor != 1.5 {
		t.Errorf("BackoffFactor = %f, want 1.5", r.BackoffFactor)
	}
	if r.JitterFactor != 0.3 {
		t.Errorf("JitterFactor = %f, want 0.3", r.JitterFactor)
	}
}

func TestBuildProviderConfig_ZeroValueOverride(t *testing.T) {
	raw := config.RawProviderConfig{
		Type:   "groq",
		APIKey: "sk-groq",
		Resilience: &config.RawResilienceConfig{
			Retry: &config.RawRetryConfig{
				MaxRetries: intPtr(0),
			},
		},
	}
	got := buildProviderConfig(raw, globalResilience)

	if got.Resilience.Retry.MaxRetries != 0 {
		t.Errorf("explicit 0 should override global (3), got %d", got.Resilience.Retry.MaxRetries)
	}
}

func TestBuildProviderConfig_PreservesFields(t *testing.T) {
	raw := config.RawProviderConfig{
		Type:    "openai",
		APIKey:  "sk-key",
		BaseURL: "https://custom.endpoint.com",
		Models:  []string{"gpt-4", "gpt-3.5-turbo"},
	}
	got := buildProviderConfig(raw, globalResilience)

	if got.APIKey != "sk-key" {
		t.Errorf("APIKey = %q, want sk-key", got.APIKey)
	}
	if got.BaseURL != "https://custom.endpoint.com" {
		t.Errorf("BaseURL = %q, want https://custom.endpoint.com", got.BaseURL)
	}
	if len(got.Models) != 2 || got.Models[0] != "gpt-4" {
		t.Errorf("Models = %v, want [gpt-4 gpt-3.5-turbo]", got.Models)
	}
}

// --- buildProviderConfigs ---

func TestBuildProviderConfigs_MultipleProviders(t *testing.T) {
	maxRetries := 10
	raw := map[string]config.RawProviderConfig{
		"openai": {
			Type:   "openai",
			APIKey: "sk-openai",
			Resilience: &config.RawResilienceConfig{
				Retry: &config.RawRetryConfig{MaxRetries: &maxRetries},
			},
		},
		"anthropic": {Type: "anthropic", APIKey: "sk-ant"},
	}

	got := buildProviderConfigs(raw, globalResilience)

	if got["openai"].Resilience.Retry.MaxRetries != 10 {
		t.Errorf("openai MaxRetries = %d, want 10", got["openai"].Resilience.Retry.MaxRetries)
	}
	if got["anthropic"].Resilience.Retry.MaxRetries != globalRetry.MaxRetries {
		t.Errorf("anthropic MaxRetries = %d, want %d (global)", got["anthropic"].Resilience.Retry.MaxRetries, globalRetry.MaxRetries)
	}
}

func TestBuildProviderConfigs_EmptyMap(t *testing.T) {
	got := buildProviderConfigs(map[string]config.RawProviderConfig{}, globalResilience)
	if len(got) != 0 {
		t.Errorf("expected empty result, got %d entries", len(got))
	}
}

// --- filterEmptyProviders ---

func TestFilterEmptyProviders_RemovesEmptyAPIKey(t *testing.T) {
	raw := map[string]config.RawProviderConfig{
		"openai":    {Type: "openai", APIKey: ""},
		"anthropic": {Type: "anthropic", APIKey: "sk-ant"},
	}
	got := filterEmptyProviders(raw)

	if _, exists := got["openai"]; exists {
		t.Error("expected openai with empty API key to be removed")
	}
	if _, exists := got["anthropic"]; !exists {
		t.Error("expected anthropic to be kept")
	}
}

func TestFilterEmptyProviders_RemovesUnresolvedPlaceholder(t *testing.T) {
	raw := map[string]config.RawProviderConfig{
		"openai":    {Type: "openai", APIKey: "${OPENAI_API_KEY}"},
		"anthropic": {Type: "anthropic", APIKey: "sk-real"},
	}
	got := filterEmptyProviders(raw)

	if _, exists := got["openai"]; exists {
		t.Error("expected openai with unresolved placeholder to be removed")
	}
	if _, exists := got["anthropic"]; !exists {
		t.Error("expected anthropic to survive filtering")
	}
}

func TestFilterEmptyProviders_RemovesPartialPlaceholder(t *testing.T) {
	raw := map[string]config.RawProviderConfig{
		"openai": {Type: "openai", APIKey: "prefix-${UNRESOLVED}"},
	}
	got := filterEmptyProviders(raw)

	if _, exists := got["openai"]; exists {
		t.Error("expected provider with partial placeholder to be removed")
	}
}

func TestFilterEmptyProviders_OllamaExemptWithBaseURL(t *testing.T) {
	raw := map[string]config.RawProviderConfig{
		"ollama": {Type: "ollama", APIKey: "", BaseURL: "http://localhost:11434/v1"},
	}
	got := filterEmptyProviders(raw)

	if _, exists := got["ollama"]; !exists {
		t.Error("expected ollama with BaseURL to be kept without API key")
	}
}

func TestFilterEmptyProviders_OllamaRemovedWithoutBaseURL(t *testing.T) {
	raw := map[string]config.RawProviderConfig{
		"ollama": {Type: "ollama", APIKey: "", BaseURL: ""},
	}
	got := filterEmptyProviders(raw)

	if _, exists := got["ollama"]; exists {
		t.Error("expected ollama without BaseURL to be removed")
	}
}

func TestFilterEmptyProviders_OllamaWithBothAPIKeyAndBaseURL(t *testing.T) {
	raw := map[string]config.RawProviderConfig{
		"ollama": {Type: "ollama", APIKey: "sk-ollama", BaseURL: "http://localhost:11434/v1"},
	}
	got := filterEmptyProviders(raw)

	if _, exists := got["ollama"]; !exists {
		t.Error("expected ollama with both key and URL to be kept")
	}
}

func TestFilterEmptyProviders_EmptyMap(t *testing.T) {
	got := filterEmptyProviders(map[string]config.RawProviderConfig{})
	if len(got) != 0 {
		t.Errorf("expected empty result, got %d entries", len(got))
	}
}

// --- applyProviderEnvVars ---

func TestApplyProviderEnvVars_DiscoversFromAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-from-env")

	got := applyProviderEnvVars(map[string]config.RawProviderConfig{})

	p, exists := got["openai"]
	if !exists {
		t.Fatal("expected openai to be discovered from env var")
	}
	if p.APIKey != "sk-from-env" {
		t.Errorf("APIKey = %q, want sk-from-env", p.APIKey)
	}
	if p.Type != "openai" {
		t.Errorf("Type = %q, want openai", p.Type)
	}
}

func TestApplyProviderEnvVars_DiscoversFromBaseURL(t *testing.T) {
	t.Setenv("OLLAMA_BASE_URL", "http://localhost:11434/v1")

	got := applyProviderEnvVars(map[string]config.RawProviderConfig{})

	p, exists := got["ollama"]
	if !exists {
		t.Fatal("expected ollama to be discovered from base URL env var")
	}
	if p.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("BaseURL = %q, want http://localhost:11434/v1", p.BaseURL)
	}
}

func TestApplyProviderEnvVars_EnvWinsOverYAML(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-env-key")

	raw := map[string]config.RawProviderConfig{
		"openai": {Type: "openai", APIKey: "sk-yaml-key", BaseURL: "https://custom.api.com"},
	}
	got := applyProviderEnvVars(raw)

	if got["openai"].APIKey != "sk-env-key" {
		t.Errorf("APIKey = %q, want sk-env-key (env should win over YAML)", got["openai"].APIKey)
	}
	if got["openai"].BaseURL != "https://custom.api.com" {
		t.Error("BaseURL from YAML should be preserved when env var is absent")
	}
}

func TestApplyProviderEnvVars_BaseURLEnvWinsOverYAML(t *testing.T) {
	t.Setenv("OPENAI_BASE_URL", "https://env-override.com")

	raw := map[string]config.RawProviderConfig{
		"openai": {Type: "openai", APIKey: "sk-key", BaseURL: "https://yaml-url.com"},
	}
	got := applyProviderEnvVars(raw)

	if got["openai"].BaseURL != "https://env-override.com" {
		t.Errorf("BaseURL = %q, want https://env-override.com", got["openai"].BaseURL)
	}
}

func TestApplyProviderEnvVars_PreservesYAMLResilience(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-env-key")

	maxRetries := 10
	raw := map[string]config.RawProviderConfig{
		"openai": {
			Type:   "openai",
			APIKey: "sk-yaml-key",
			Resilience: &config.RawResilienceConfig{
				Retry: &config.RawRetryConfig{MaxRetries: &maxRetries},
			},
		},
	}
	got := applyProviderEnvVars(raw)

	if got["openai"].Resilience == nil || got["openai"].Resilience.Retry == nil {
		t.Fatal("expected YAML resilience to be preserved after env var overlay")
	}
	if *got["openai"].Resilience.Retry.MaxRetries != 10 {
		t.Errorf("MaxRetries = %d, want 10", *got["openai"].Resilience.Retry.MaxRetries)
	}
}

func TestApplyProviderEnvVars_SkipsWhenNoEnvVars(t *testing.T) {
	got := applyProviderEnvVars(map[string]config.RawProviderConfig{})
	if len(got) != 0 {
		t.Errorf("expected empty result when no env vars set, got %d entries", len(got))
	}
}

func TestApplyProviderEnvVars_PreservesUnknownYAMLProviders(t *testing.T) {
	raw := map[string]config.RawProviderConfig{
		"custom-provider": {Type: "custom", APIKey: "sk-custom"},
	}
	got := applyProviderEnvVars(raw)

	if _, exists := got["custom-provider"]; !exists {
		t.Error("expected custom (non-knownProviderEnvs) YAML provider to be preserved")
	}
}

// --- resolveProviders (integration of all three stages) ---

func TestResolveProviders_EndToEnd(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-env")

	maxRetries := 10
	raw := map[string]config.RawProviderConfig{
		"openai": {
			Type:   "openai",
			APIKey: "sk-openai-yaml",
			Resilience: &config.RawResilienceConfig{
				Retry: &config.RawRetryConfig{MaxRetries: &maxRetries},
			},
		},
		"bad": {
			Type:   "openai",
			APIKey: "${UNRESOLVED}",
		},
	}

	got := resolveProviders(raw, globalResilience)

	if _, exists := got["bad"]; exists {
		t.Error("expected provider with unresolved placeholder to be filtered out")
	}
	if got["openai"].Resilience.Retry.MaxRetries != 10 {
		t.Errorf("openai MaxRetries = %d, want 10", got["openai"].Resilience.Retry.MaxRetries)
	}
	if got["anthropic"].APIKey != "sk-ant-env" {
		t.Errorf("anthropic APIKey = %q, want sk-ant-env", got["anthropic"].APIKey)
	}
	if got["anthropic"].Resilience.Retry.MaxRetries != globalRetry.MaxRetries {
		t.Errorf("anthropic should inherit global MaxRetries=%d, got %d", globalRetry.MaxRetries, got["anthropic"].Resilience.Retry.MaxRetries)
	}
}

func TestResolveProviders_EmptyRaw_OnlyEnvVars(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "sk-groq")

	got := resolveProviders(map[string]config.RawProviderConfig{}, globalResilience)

	if got["groq"].APIKey != "sk-groq" {
		t.Errorf("groq APIKey = %q, want sk-groq", got["groq"].APIKey)
	}
}

func TestResolveProviders_NoProvidersNoEnvVars(t *testing.T) {
	got := resolveProviders(map[string]config.RawProviderConfig{}, globalResilience)
	if len(got) != 0 {
		t.Errorf("expected empty result, got %d entries", len(got))
	}
}
