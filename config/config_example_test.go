package config

import (
	"os"
	"testing"
)

func TestLoad_FromEnvironment(t *testing.T) {
	// Set up environment variables
	_ = os.Setenv("PORT", "9090")
	_ = os.Setenv("OPENAI_API_KEY", "test-openai-key")
	_ = os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	defer func() {
		_ = os.Unsetenv("PORT")
		_ = os.Unsetenv("OPENAI_API_KEY")
		_ = os.Unsetenv("ANTHROPIC_API_KEY")
	}()

	// Note: This test assumes config.yaml exists and uses ${VAR} placeholders
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// When config.yaml exists with hardcoded port, it takes precedence
	// In production, use ${PORT} in config.yaml to allow env var override
	if cfg.Server.Port == "" {
		t.Error("expected non-empty port")
	}

	// Providers should be created from expanded env vars
	if len(cfg.Providers) < 2 {
		t.Errorf("expected at least 2 providers, got %d", len(cfg.Providers))
	}

	// Check OpenAI provider
	if openaiCfg, exists := cfg.Providers["openai"]; exists {
		if openaiCfg.Type != "openai" {
			t.Errorf("expected openai type, got '%s'", openaiCfg.Type)
		}
		if openaiCfg.APIKey != "test-openai-key" {
			t.Errorf("expected openai key 'test-openai-key', got '%s'", openaiCfg.APIKey)
		}
	} else {
		t.Error("expected 'openai' provider to exist")
	}

	// Check Anthropic provider
	if anthropicCfg, exists := cfg.Providers["anthropic"]; exists {
		if anthropicCfg.Type != "anthropic" {
			t.Errorf("expected anthropic type, got '%s'", anthropicCfg.Type)
		}
		if anthropicCfg.APIKey != "test-anthropic-key" {
			t.Errorf("expected anthropic key 'test-anthropic-key', got '%s'", anthropicCfg.APIKey)
		}
	} else {
		t.Error("expected 'anthropic' provider to exist")
	}
}

func TestProviderConfig_Fields(t *testing.T) {
	// Test that ProviderConfig has all expected fields
	cfg := ProviderConfig{
		Type:    "openai",
		APIKey:  "test-key",
		BaseURL: "https://custom.endpoint.com",
		Models:  []string{"gpt-4", "gpt-3.5-turbo"},
	}

	if cfg.Type != "openai" {
		t.Errorf("expected type 'openai', got '%s'", cfg.Type)
	}
	if cfg.APIKey != "test-key" {
		t.Errorf("expected api_key 'test-key', got '%s'", cfg.APIKey)
	}
	if cfg.BaseURL != "https://custom.endpoint.com" {
		t.Errorf("expected base_url 'https://custom.endpoint.com', got '%s'", cfg.BaseURL)
	}
	if len(cfg.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(cfg.Models))
	}
}

func TestConfig_ProvidersMap(t *testing.T) {
	// Test that Config can hold multiple providers
	cfg := Config{
		Server: ServerConfig{Port: "8080"},
		Providers: map[string]ProviderConfig{
			"openai-1": {
				Type:   "openai",
				APIKey: "key1",
			},
			"openai-2": {
				Type:    "openai",
				APIKey:  "key2",
				BaseURL: "https://custom.endpoint.com",
			},
			"anthropic": {
				Type:   "anthropic",
				APIKey: "key3",
			},
		},
	}

	if len(cfg.Providers) != 3 {
		t.Errorf("expected 3 providers, got %d", len(cfg.Providers))
	}

	// Verify we can have multiple providers of the same type
	openai1, exists := cfg.Providers["openai-1"]
	if !exists {
		t.Error("expected 'openai-1' provider to exist")
	}
	if openai1.BaseURL != "" {
		t.Error("expected openai-1 to have empty base_url")
	}

	openai2, exists := cfg.Providers["openai-2"]
	if !exists {
		t.Error("expected 'openai-2' provider to exist")
	}
	if openai2.BaseURL != "https://custom.endpoint.com" {
		t.Errorf("expected openai-2 to have custom base_url, got '%s'", openai2.BaseURL)
	}
}
