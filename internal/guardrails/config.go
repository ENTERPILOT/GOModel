// Package guardrails provides request preprocessing features including
// system prompt injection and PII anonymization.
package guardrails

// Config holds all guardrails configuration.
type Config struct {
	SystemPrompt  SystemPromptConfig  `mapstructure:"system_prompt"`
	Anonymization AnonymizationConfig `mapstructure:"anonymization"`
}

// SystemPromptConfig configures system prompt injection behavior.
type SystemPromptConfig struct {
	// Enabled controls whether system prompt injection is active.
	Enabled bool `mapstructure:"enabled"`

	// Global defines the default system prompt applied to all requests.
	Global *SystemPromptRule `mapstructure:"global"`

	// Models defines model-specific system prompt rules (highest precedence).
	// Map key is the model name (e.g., "gpt-4").
	Models map[string]SystemPromptRule `mapstructure:"models"`

	// Providers defines provider-specific system prompt rules.
	// Map key is the provider type (e.g., "anthropic").
	Providers map[string]SystemPromptRule `mapstructure:"providers"`
}

// SystemPromptRule defines how to inject a system prompt.
type SystemPromptRule struct {
	// Prompt is the system prompt text to inject.
	Prompt string `mapstructure:"prompt"`

	// Position specifies where to inject the prompt: "prepend", "append", or "replace".
	// Default: "prepend"
	Position string `mapstructure:"position"`

	// PreserveUserSystem controls whether to keep the user's existing system message
	// when position is "prepend" or "append". Ignored for "replace".
	// Default: true
	PreserveUserSystem bool `mapstructure:"preserve_user_system"`
}

// PositionType constants for system prompt injection positions.
const (
	PositionPrepend = "prepend"
	PositionAppend  = "append"
	PositionReplace = "replace"
)

// AnonymizationConfig configures PII detection and anonymization.
type AnonymizationConfig struct {
	// Enabled controls whether anonymization is active.
	Enabled bool `mapstructure:"enabled"`

	// Models is a whitelist of model names that require anonymization.
	// If empty and Enabled is true, anonymization applies to all models.
	Models []string `mapstructure:"models"`

	// Detectors controls which PII types to detect.
	Detectors DetectorConfig `mapstructure:"detectors"`

	// Strategy defines how to anonymize detected PII: "token", "hash", or "mask".
	// Default: "token"
	Strategy string `mapstructure:"strategy"`

	// DeanonymizeResponses controls whether to restore original values in responses.
	// Default: true
	DeanonymizeResponses bool `mapstructure:"deanonymize_responses"`
}

// DetectorConfig controls which PII detectors are enabled.
type DetectorConfig struct {
	Email      bool `mapstructure:"email"`
	Phone      bool `mapstructure:"phone"`
	SSN        bool `mapstructure:"ssn"`
	CreditCard bool `mapstructure:"credit_card"`
	IPAddress  bool `mapstructure:"ip_address"`
}

// Strategy constants for anonymization strategies.
const (
	StrategyToken = "token"
	StrategyHash  = "hash"
	StrategyMask  = "mask"
)

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		SystemPrompt: SystemPromptConfig{
			Enabled:   false,
			Models:    make(map[string]SystemPromptRule),
			Providers: make(map[string]SystemPromptRule),
		},
		Anonymization: AnonymizationConfig{
			Enabled:  false,
			Models:   nil,
			Strategy: StrategyToken,
			Detectors: DetectorConfig{
				Email:      true,
				Phone:      true,
				SSN:        true,
				CreditCard: true,
				IPAddress:  true,
			},
			DeanonymizeResponses: true,
		},
	}
}
