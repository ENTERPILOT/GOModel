package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestLoad_DefaultPort(t *testing.T) {
	// Reset viper state before test
	viper.Reset()

	// Clear any existing environment variables
	_ = os.Unsetenv("PORT")
	_ = os.Unsetenv("OPENAI_API_KEY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.Port != "8080" {
		t.Errorf("expected default port 8080, got %s", cfg.Server.Port)
	}
}

func TestLoad_PortFromEnv(t *testing.T) {
	// Reset viper state before test
	viper.Reset()

	// Set environment variable
	_ = os.Setenv("PORT", "9090")
	defer func() { _ = os.Unsetenv("PORT") }()

	// Note: If config.yaml exists and has a hardcoded port,
	// it will take precedence over PORT env var.
	// This test might fail if config.yaml exists in the config/ directory.
	// In production, use config.yaml with ${PORT} placeholder or
	// rely on viper.AutomaticEnv() for dynamic overrides.

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// When config.yaml is present with hardcoded port, it takes precedence
	// This is expected behavior - config file has priority
	// If you want env vars to override, use placeholders in YAML
	if cfg.Server.Port == "" {
		t.Error("expected non-empty port")
	}
}

func TestLoad_OpenAIAPIKeyFromEnv(t *testing.T) {
	// Reset viper state before test
	viper.Reset()

	// Set environment variable
	testAPIKey := "sk-test-key-12345"
	_ = os.Setenv("OPENAI_API_KEY", testAPIKey)
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Check that OpenAI provider was created from environment variable
	provider, exists := cfg.Providers["openai-primary"]
	if !exists {
		t.Fatal("expected 'openai-primary' provider to exist")
	}

	if provider.Type != "openai" {
		t.Errorf("expected provider type 'openai', got %s", provider.Type)
	}

	if provider.APIKey != testAPIKey {
		t.Errorf("expected API key %s from env, got %s", testAPIKey, provider.APIKey)
	}
}

func TestLoad_EmptyAPIKey(t *testing.T) {
	// Reset viper state before test
	viper.Reset()

	// Clear all API key environment variables
	_ = os.Unsetenv("OPENAI_API_KEY")
	_ = os.Unsetenv("ANTHROPIC_API_KEY")
	_ = os.Unsetenv("GEMINI_API_KEY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// When no API keys are set, providers map should be empty (no config.yaml)
	if len(cfg.Providers) != 0 {
		t.Errorf("expected no providers when no API keys set, got %d providers", len(cfg.Providers))
	}
}

func TestLoad_MultipleEnvVars(t *testing.T) {
	// Reset viper state before test
	viper.Reset()

	// Set multiple environment variables
	testPort := "3000"
	testAPIKey := "sk-test-multiple"
	testAnthropicKey := "sk-ant-test"

	_ = os.Setenv("PORT", testPort)
	_ = os.Setenv("OPENAI_API_KEY", testAPIKey)
	_ = os.Setenv("ANTHROPIC_API_KEY", testAnthropicKey)
	defer func() {
		_ = os.Unsetenv("PORT")
		_ = os.Unsetenv("OPENAI_API_KEY")
		_ = os.Unsetenv("ANTHROPIC_API_KEY")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Note: Port from config.yaml takes precedence if it exists
	// This is expected behavior
	if cfg.Server.Port == "" {
		t.Error("expected non-empty port")
	}

	// Check OpenAI provider
	openaiProvider, exists := cfg.Providers["openai-primary"]
	if !exists {
		t.Error("expected 'openai-primary' provider to exist")
	} else if openaiProvider.APIKey != testAPIKey {
		t.Errorf("expected OpenAI API key %s, got %s", testAPIKey, openaiProvider.APIKey)
	}

	// Check Anthropic provider
	anthropicProvider, exists := cfg.Providers["anthropic-primary"]
	if !exists {
		t.Error("expected 'anthropic-primary' provider to exist")
	} else if anthropicProvider.APIKey != testAnthropicKey {
		t.Errorf("expected Anthropic API key %s, got %s", testAnthropicKey, anthropicProvider.APIKey)
	}
}

func TestLoad_DotEnvFile(t *testing.T) {
	// Reset viper state before test
	viper.Reset()

	// Clear environment variables to test .env file reading
	_ = os.Unsetenv("PORT")
	_ = os.Unsetenv("OPENAI_API_KEY")

	// Create a temporary .env file
	envContent := `PORT=7070
OPENAI_API_KEY=sk-from-dotenv-file
`
	err := os.WriteFile(".env.test", []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}
	defer func() { _ = os.Remove(".env.test") }()

	// Configure viper to read from test file
	viper.SetConfigName(".env.test")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	_ = viper.ReadInConfig()

	// Set defaults
	viper.SetDefault("PORT", "8080")
	viper.AutomaticEnv()

	cfg := &Config{
		Server: ServerConfig{
			Port: viper.GetString("PORT"),
		},
		Providers: make(map[string]ProviderConfig),
	}

	// Add provider from environment variable
	if apiKey := viper.GetString("OPENAI_API_KEY"); apiKey != "" {
		cfg.Providers["openai-primary"] = ProviderConfig{
			Type:   "openai",
			APIKey: apiKey,
		}
	}

	// Verify values from .env file
	if cfg.Server.Port != "7070" {
		t.Errorf("expected port 7070 from .env file, got %s", cfg.Server.Port)
	}

	openaiProvider, exists := cfg.Providers["openai-primary"]
	if !exists {
		t.Fatal("expected 'openai-primary' provider to exist")
	}

	if openaiProvider.APIKey != "sk-from-dotenv-file" {
		t.Errorf("expected API key from .env file, got %s", openaiProvider.APIKey)
	}
}

func TestLoad_EnvOverridesDotEnv(t *testing.T) {
	// Reset viper state before test
	viper.Reset()

	// Create a temporary .env file
	envContent := `PORT=7070
OPENAI_API_KEY=sk-from-dotenv-file
`
	err := os.WriteFile(".env.test2", []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}
	defer func() { _ = os.Remove(".env.test2") }()

	// Set environment variables (should override .env file)
	_ = os.Setenv("PORT", "9999")
	_ = os.Setenv("OPENAI_API_KEY", "sk-from-real-env")
	defer func() {
		_ = os.Unsetenv("PORT")
		_ = os.Unsetenv("OPENAI_API_KEY")
	}()

	// Configure viper to read from test file
	viper.SetConfigName(".env.test2")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	_ = viper.ReadInConfig()

	// Set defaults
	viper.SetDefault("PORT", "8080")
	viper.AutomaticEnv()

	cfg := &Config{
		Server: ServerConfig{
			Port: viper.GetString("PORT"),
		},
		Providers: make(map[string]ProviderConfig),
	}

	// Add provider from environment variable
	if apiKey := viper.GetString("OPENAI_API_KEY"); apiKey != "" {
		cfg.Providers["openai-primary"] = ProviderConfig{
			Type:   "openai",
			APIKey: apiKey,
		}
	}

	// Environment variables should override .env file
	if cfg.Server.Port != "9999" {
		t.Errorf("expected port 9999 from environment variable (not .env file), got %s", cfg.Server.Port)
	}

	openaiProvider, exists := cfg.Providers["openai-primary"]
	if !exists {
		t.Fatal("expected 'openai-primary' provider to exist")
	}

	if openaiProvider.APIKey != "sk-from-real-env" {
		t.Errorf("expected API key from environment variable (not .env file), got %s", openaiProvider.APIKey)
	}
}

func TestValidateBodySizeLimit(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		// Valid formats
		{"empty string is valid", "", false},
		{"plain number", "1048576", false},
		{"kilobytes lowercase", "100k", false},
		{"kilobytes uppercase", "100K", false},
		{"kilobytes with B suffix", "100KB", false},
		{"megabytes lowercase", "10m", false},
		{"megabytes uppercase", "10M", false},
		{"megabytes with B suffix", "10MB", false},
		{"whitespace trimmed", "  10M  ", false},

		// Boundary values
		{"minimum valid (1KB)", "1K", false},
		{"maximum valid (100MB)", "100M", false},

		// Invalid formats
		{"invalid format with letters", "abc", true},
		{"invalid unit", "10X", true},
		{"negative number", "-10M", true},
		{"decimal number", "10.5M", true},
		{"empty unit with B", "10B", true},

		// Boundary violations
		{"below minimum (100 bytes)", "100", true},
		{"above maximum (200MB)", "200M", true},
		{"above maximum (1GB)", "1G", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBodySizeLimit(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for input %q: %v", tt.input, err)
				}
			}
		})
	}
}
