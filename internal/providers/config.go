package providers

import (
	"os"
	"strings"

	"gomodel/config"
)

// ProviderConfig holds the fully resolved provider configuration after merging
// global defaults with per-provider overrides.
type ProviderConfig struct {
	Type       string                 `yaml:"type"`
	APIKey     string                 `yaml:"api_key"`
	BaseURL    string                 `yaml:"base_url"`
	Models     []string               `yaml:"models"`
	Resilience config.ResilienceConfig `yaml:"resilience"`
}

// knownProviderEnvs maps well-known provider names to their environment variables.
// This list is the authoritative source for provider auto-discovery from env vars.
var knownProviderEnvs = []struct {
	name         string
	providerType string
	apiKeyEnv    string
	baseURLEnv   string
}{
	{"openai", "openai", "OPENAI_API_KEY", "OPENAI_BASE_URL"},
	{"anthropic", "anthropic", "ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL"},
	{"gemini", "gemini", "GEMINI_API_KEY", "GEMINI_BASE_URL"},
	{"xai", "xai", "XAI_API_KEY", "XAI_BASE_URL"},
	{"groq", "groq", "GROQ_API_KEY", "GROQ_BASE_URL"},
	{"ollama", "ollama", "OLLAMA_API_KEY", "OLLAMA_BASE_URL"},
}

// resolveProviders applies env var overrides to the raw YAML provider map, filters
// out entries with invalid credentials, and merges each entry with the global
// ResilienceConfig. Returns a fully resolved map ready for provider instantiation.
func resolveProviders(raw map[string]config.RawProviderConfig, global config.ResilienceConfig) map[string]ProviderConfig {
	merged := applyProviderEnvVars(raw)
	filtered := filterEmptyProviders(merged)
	return buildProviderConfigs(filtered, global)
}

// applyProviderEnvVars overlays well-known provider env vars onto the raw YAML map.
// Env var values always win over YAML values for the same provider name.
func applyProviderEnvVars(raw map[string]config.RawProviderConfig) map[string]config.RawProviderConfig {
	result := make(map[string]config.RawProviderConfig, len(raw))
	for k, v := range raw {
		result[k] = v
	}

	for _, kp := range knownProviderEnvs {
		apiKey := os.Getenv(kp.apiKeyEnv)
		baseURL := os.Getenv(kp.baseURLEnv)

		if apiKey == "" && baseURL == "" {
			continue
		}

		existing, exists := result[kp.name]
		if exists {
			if apiKey != "" {
				existing.APIKey = apiKey
			}
			if baseURL != "" {
				existing.BaseURL = baseURL
			}
			result[kp.name] = existing
		} else {
			result[kp.name] = config.RawProviderConfig{
				Type:    kp.providerType,
				APIKey:  apiKey,
				BaseURL: baseURL,
			}
		}
	}

	return result
}

// filterEmptyProviders removes providers without valid credentials.
// Ollama is exempt from the API key requirement if it has a BaseURL.
func filterEmptyProviders(raw map[string]config.RawProviderConfig) map[string]config.RawProviderConfig {
	result := make(map[string]config.RawProviderConfig, len(raw))
	for name, p := range raw {
		if p.Type == "ollama" && p.BaseURL != "" {
			result[name] = p
			continue
		}
		if p.APIKey != "" && !strings.Contains(p.APIKey, "${") {
			result[name] = p
		}
	}
	return result
}

// buildProviderConfigs merges each raw provider config with the global ResilienceConfig,
// producing fully resolved ProviderConfig values.
func buildProviderConfigs(raw map[string]config.RawProviderConfig, global config.ResilienceConfig) map[string]ProviderConfig {
	result := make(map[string]ProviderConfig, len(raw))
	for name, r := range raw {
		result[name] = buildProviderConfig(r, global)
	}
	return result
}

// buildProviderConfig merges a single RawProviderConfig with the global ResilienceConfig.
// Non-nil fields in the raw config override the global defaults.
func buildProviderConfig(raw config.RawProviderConfig, global config.ResilienceConfig) ProviderConfig {
	resolved := ProviderConfig{
		Type:       raw.Type,
		APIKey:     raw.APIKey,
		BaseURL:    raw.BaseURL,
		Models:     raw.Models,
		Resilience: global,
	}

	if raw.Resilience == nil || raw.Resilience.Retry == nil {
		return resolved
	}

	r := raw.Resilience.Retry
	if r.MaxRetries != nil {
		resolved.Resilience.Retry.MaxRetries = *r.MaxRetries
	}
	if r.InitialBackoff != nil {
		resolved.Resilience.Retry.InitialBackoff = *r.InitialBackoff
	}
	if r.MaxBackoff != nil {
		resolved.Resilience.Retry.MaxBackoff = *r.MaxBackoff
	}
	if r.BackoffFactor != nil {
		resolved.Resilience.Retry.BackoffFactor = *r.BackoffFactor
	}
	if r.JitterFactor != nil {
		resolved.Resilience.Retry.JitterFactor = *r.JitterFactor
	}

	return resolved
}
