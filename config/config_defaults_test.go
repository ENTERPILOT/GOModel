package config

import (
	"os"
	"testing"

	viper "github.com/spf13/viper"
)

func TestLoad_WithDefaults(t *testing.T) {
	// 1. Test Default Value
	t.Run("UseDefaultValue", func(t *testing.T) {
		// Create config with default value syntax
		configContent := `
server:
  port: "${TEST_PORT_DEFAULTS:-9999}"
providers:
  openai-primary:
    type: "openai"
    api_key: "${TEST_KEY_DEFAULTS:-default-key}"
`
		err := os.WriteFile("config.yaml", []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}
		defer os.Remove("config.yaml")

		// Ensure env vars are unset
		os.Unsetenv("TEST_PORT_DEFAULTS")
		os.Unsetenv("TEST_KEY_DEFAULTS")
		defer os.Unsetenv("TEST_PORT_DEFAULTS")
		defer os.Unsetenv("TEST_KEY_DEFAULTS")

		// Reset viper
		viper.Reset()

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if cfg.Server.Port != "9999" {
			t.Errorf("Expected port 9999 (default), got %s", cfg.Server.Port)
		}

		provider := cfg.Providers["openai-primary"]
		if provider.APIKey != "default-key" {
			t.Errorf("Expected API key 'default-key', got %s", provider.APIKey)
		}
	})

	// 2. Test Env Var Override
	t.Run("OverrideDefaultValue", func(t *testing.T) {
		// Same config content...
		// But set env vars
		os.Setenv("TEST_PORT_DEFAULTS", "1111")
		os.Setenv("TEST_KEY_DEFAULTS", "real-key")
		defer os.Unsetenv("TEST_PORT_DEFAULTS")
		defer os.Unsetenv("TEST_KEY_DEFAULTS")

		// Create config (need to recreate as Load might re-read)
		configContent := `
server:
  port: "${TEST_PORT_DEFAULTS:-9999}"
providers:
  openai-primary:
    type: "openai"
    api_key: "${TEST_KEY_DEFAULTS:-default-key}"
`
		err := os.WriteFile("config.yaml", []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}
		defer os.Remove("config.yaml")

		viper.Reset()

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if cfg.Server.Port != "1111" {
			t.Errorf("Expected port 1111 (env override), got %s", cfg.Server.Port)
		}

		provider := cfg.Providers["openai-primary"]
		if provider.APIKey != "real-key" {
			t.Errorf("Expected API key 'real-key', got %s", provider.APIKey)
		}
	})
}
