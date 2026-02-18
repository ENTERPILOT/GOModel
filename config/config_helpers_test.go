package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestExpandString tests the expandString function with various scenarios
func TestExpandString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			envVars:  map[string]string{},
			expected: "",
		},
		{
			name:     "string without placeholders",
			input:    "simple-string",
			envVars:  map[string]string{},
			expected: "simple-string",
		},
		{
			name:     "simple variable expansion",
			input:    "${API_KEY}",
			envVars:  map[string]string{"API_KEY": "sk-12345"},
			expected: "sk-12345",
		},
		{
			name:     "variable in middle of string",
			input:    "prefix-${API_KEY}-suffix",
			envVars:  map[string]string{"API_KEY": "sk-12345"},
			expected: "prefix-sk-12345-suffix",
		},
		{
			name:     "multiple variables",
			input:    "${SCHEME}://${HOST}:${PORT}",
			envVars:  map[string]string{"SCHEME": "https", "HOST": "api.example.com", "PORT": "8080"},
			expected: "https://api.example.com:8080",
		},
		{
			name:     "variable with default value - env var exists",
			input:    "${API_KEY:-default-key}",
			envVars:  map[string]string{"API_KEY": "sk-real-key"},
			expected: "sk-real-key",
		},
		{
			name:     "variable with default value - env var missing",
			input:    "${API_KEY:-default-key}",
			envVars:  map[string]string{},
			expected: "default-key",
		},
		{
			name:     "variable with default value - env var empty",
			input:    "${API_KEY:-default-key}",
			envVars:  map[string]string{"API_KEY": ""},
			expected: "default-key",
		},
		{
			name:     "unresolved variable - no default",
			input:    "${MISSING_VAR}",
			envVars:  map[string]string{},
			expected: "${MISSING_VAR}",
		},
		{
			name:     "partially resolved string",
			input:    "${RESOLVED}-${UNRESOLVED}",
			envVars:  map[string]string{"RESOLVED": "value1"},
			expected: "value1-${UNRESOLVED}",
		},
		{
			name:     "mixed resolved and unresolved with defaults",
			input:    "${RESOLVED}:${UNRESOLVED:-fallback}:${MISSING}",
			envVars:  map[string]string{"RESOLVED": "value1"},
			expected: "value1:fallback:${MISSING}",
		},
		{
			name:     "default value with special characters",
			input:    "${API_KEY:-https://api.example.com/v1}",
			envVars:  map[string]string{},
			expected: "https://api.example.com/v1",
		},
		{
			name:     "default value with colon in it",
			input:    "${URL:-http://localhost:8080}",
			envVars:  map[string]string{},
			expected: "http://localhost:8080",
		},
		{
			name:     "complex real-world example",
			input:    "${BASE_URL:-https://api.openai.com}/v1/chat/completions",
			envVars:  map[string]string{},
			expected: "https://api.openai.com/v1/chat/completions",
		},
		{
			name:     "environment variable set to empty string (no default)",
			input:    "${EMPTY_VAR}",
			envVars:  map[string]string{"EMPTY_VAR": ""},
			expected: "${EMPTY_VAR}",
		},
		{
			name:     "empty default value - env var missing",
			input:    "${OPTIONAL_VAR:-}",
			envVars:  map[string]string{},
			expected: "",
		},
		{
			name:     "empty default value - env var set",
			input:    "${OPTIONAL_VAR:-}",
			envVars:  map[string]string{"OPTIONAL_VAR": "actual-value"},
			expected: "actual-value",
		},
		{
			name:     "empty default value - env var empty",
			input:    "${OPTIONAL_VAR:-}",
			envVars:  map[string]string{"OPTIONAL_VAR": ""},
			expected: "",
		},
		{
			name:     "master key pattern - not set should be empty",
			input:    "${GOMODEL_MASTER_KEY:-}",
			envVars:  map[string]string{},
			expected: "",
		},
		{
			name:     "master key pattern - set to value",
			input:    "${GOMODEL_MASTER_KEY:-}",
			envVars:  map[string]string{"GOMODEL_MASTER_KEY": "secret-key"},
			expected: "secret-key",
		},
		{
			name:     "multiple placeholders some resolved some not",
			input:    "prefix-${VAR1}-${VAR2}-${VAR3}-suffix",
			envVars:  map[string]string{"VAR1": "a", "VAR3": "c"},
			expected: "prefix-a-${VAR2}-c-suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variables
			for k, v := range tt.envVars {
				_ = os.Setenv(k, v)
			}
			// Cleanup after test
			defer func() {
				for k := range tt.envVars {
					_ = os.Unsetenv(k)
				}
			}()

			result := expandString(tt.input)
			if result != tt.expected {
				t.Errorf("expandString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestRemoveEmptyProviders tests the removeEmptyProviders function
func TestRemoveEmptyProviders(t *testing.T) {
	tests := []struct {
		name              string
		providers         map[string]rawProviderConfig
		expectedProviders map[string]rawProviderConfig
	}{
		{
			name: "remove provider with empty API key",
			providers: map[string]rawProviderConfig{
				"openai": {
					Type:   "openai",
					APIKey: "",
				},
				"anthropic": {
					Type:   "anthropic",
					APIKey: "sk-ant-valid",
				},
			},
			expectedProviders: map[string]rawProviderConfig{
				"anthropic": {
					Type:   "anthropic",
					APIKey: "sk-ant-valid",
				},
			},
		},
		{
			name: "remove provider with unresolved placeholder",
			providers: map[string]rawProviderConfig{
				"openai": {
					Type:   "openai",
					APIKey: "${OPENAI_API_KEY}",
				},
				"anthropic": {
					Type:   "anthropic",
					APIKey: "sk-ant-valid",
				},
			},
			expectedProviders: map[string]rawProviderConfig{
				"anthropic": {
					Type:   "anthropic",
					APIKey: "sk-ant-valid",
				},
			},
		},
		{
			name: "remove provider with partially resolved placeholder",
			providers: map[string]rawProviderConfig{
				"openai": {
					Type:   "openai",
					APIKey: "prefix-${UNRESOLVED}",
				},
				"anthropic": {
					Type:   "anthropic",
					APIKey: "sk-ant-valid",
				},
			},
			expectedProviders: map[string]rawProviderConfig{
				"anthropic": {
					Type:   "anthropic",
					APIKey: "sk-ant-valid",
				},
			},
		},
		{
			name: "keep all providers with valid API keys",
			providers: map[string]rawProviderConfig{
				"openai": {
					Type:   "openai",
					APIKey: "sk-openai-123",
				},
				"anthropic": {
					Type:   "anthropic",
					APIKey: "sk-ant-456",
				},
				"gemini": {
					Type:   "gemini",
					APIKey: "sk-gem-789",
				},
			},
			expectedProviders: map[string]rawProviderConfig{
				"openai": {
					Type:   "openai",
					APIKey: "sk-openai-123",
				},
				"anthropic": {
					Type:   "anthropic",
					APIKey: "sk-ant-456",
				},
				"gemini": {
					Type:   "gemini",
					APIKey: "sk-gem-789",
				},
			},
		},
		{
			name: "remove all providers when all have invalid keys",
			providers: map[string]rawProviderConfig{
				"openai": {
					Type:   "openai",
					APIKey: "${OPENAI_API_KEY}",
				},
				"anthropic": {
					Type:   "anthropic",
					APIKey: "",
				},
				"gemini": {
					Type:   "gemini",
					APIKey: "${GEMINI_API_KEY}",
				},
			},
			expectedProviders: map[string]rawProviderConfig{},
		},
		{
			name: "mixed valid and invalid providers",
			providers: map[string]rawProviderConfig{
				"openai": {
					Type:   "openai",
					APIKey: "sk-openai-valid",
				},
				"openai-fallback": {
					Type:   "openai",
					APIKey: "${OPENAI_FALLBACK_KEY}",
				},
				"anthropic": {
					Type:   "anthropic",
					APIKey: "",
				},
				"gemini": {
					Type:   "gemini",
					APIKey: "sk-gemini-valid",
				},
			},
			expectedProviders: map[string]rawProviderConfig{
				"openai": {
					Type:   "openai",
					APIKey: "sk-openai-valid",
				},
				"gemini": {
					Type:   "gemini",
					APIKey: "sk-gemini-valid",
				},
			},
		},
		{
			name:              "empty providers map",
			providers:         map[string]rawProviderConfig{},
			expectedProviders: map[string]rawProviderConfig{},
		},
		{
			name: "provider with valid API key but empty BaseURL should be kept",
			providers: map[string]rawProviderConfig{
				"openai": {
					Type:    "openai",
					APIKey:  "sk-openai-123",
					BaseURL: "",
				},
			},
			expectedProviders: map[string]rawProviderConfig{
				"openai": {
					Type:    "openai",
					APIKey:  "sk-openai-123",
					BaseURL: "",
				},
			},
		},
		{
			name: "provider with valid API key but unresolved BaseURL should be kept",
			providers: map[string]rawProviderConfig{
				"openai": {
					Type:    "openai",
					APIKey:  "sk-openai-123",
					BaseURL: "${CUSTOM_URL}",
				},
			},
			expectedProviders: map[string]rawProviderConfig{
				"openai": {
					Type:    "openai",
					APIKey:  "sk-openai-123",
					BaseURL: "${CUSTOM_URL}",
				},
			},
		},
		{
			name: "ollama with base URL preserved (no API key needed)",
			providers: map[string]rawProviderConfig{
				"ollama": {
					Type:    "ollama",
					APIKey:  "",
					BaseURL: "http://localhost:11434/v1",
				},
			},
			expectedProviders: map[string]rawProviderConfig{
				"ollama": {
					Type:    "ollama",
					APIKey:  "",
					BaseURL: "http://localhost:11434/v1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			removeEmptyProviders(tt.providers)

			if len(tt.providers) != len(tt.expectedProviders) {
				t.Errorf("len(Providers) = %d, want %d", len(tt.providers), len(tt.expectedProviders))
			}

			for name, expectedProvider := range tt.expectedProviders {
				resultProvider, exists := tt.providers[name]
				if !exists {
					t.Errorf("Provider %q not found in result", name)
					continue
				}

				if resultProvider.Type != expectedProvider.Type {
					t.Errorf("Provider %q: Type = %q, want %q", name, resultProvider.Type, expectedProvider.Type)
				}
				if resultProvider.APIKey != expectedProvider.APIKey {
					t.Errorf("Provider %q: APIKey = %q, want %q", name, resultProvider.APIKey, expectedProvider.APIKey)
				}
				if resultProvider.BaseURL != expectedProvider.BaseURL {
					t.Errorf("Provider %q: BaseURL = %q, want %q", name, resultProvider.BaseURL, expectedProvider.BaseURL)
				}
			}

			for name := range tt.providers {
				if _, exists := tt.expectedProviders[name]; !exists {
					t.Errorf("Unexpected provider %q found in result", name)
				}
			}
		})
	}
}

// TestApplyEnvOverrides tests the applyEnvOverrides function
func TestApplyEnvOverrides(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		check   func(t *testing.T, cfg *Config)
	}{
		{
			name:    "PORT override",
			envVars: map[string]string{"PORT": "3000"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.Server.Port != "3000" {
					t.Errorf("Server.Port = %q, want %q", cfg.Server.Port, "3000")
				}
			},
		},
		{
			name:    "GOMODEL_MASTER_KEY override",
			envVars: map[string]string{"GOMODEL_MASTER_KEY": "my-secret"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.Server.MasterKey != "my-secret" {
					t.Errorf("Server.MasterKey = %q, want %q", cfg.Server.MasterKey, "my-secret")
				}
			},
		},
		{
			name:    "storage overrides",
			envVars: map[string]string{"STORAGE_TYPE": "postgresql", "POSTGRES_URL": "postgres://localhost/test", "POSTGRES_MAX_CONNS": "20"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.Storage.Type != "postgresql" {
					t.Errorf("Storage.Type = %q, want %q", cfg.Storage.Type, "postgresql")
				}
				if cfg.Storage.PostgreSQL.URL != "postgres://localhost/test" {
					t.Errorf("Storage.PostgreSQL.URL = %q, want %q", cfg.Storage.PostgreSQL.URL, "postgres://localhost/test")
				}
				if cfg.Storage.PostgreSQL.MaxConns != 20 {
					t.Errorf("Storage.PostgreSQL.MaxConns = %d, want %d", cfg.Storage.PostgreSQL.MaxConns, 20)
				}
			},
		},
		{
			name:    "bool overrides",
			envVars: map[string]string{"METRICS_ENABLED": "true", "LOGGING_ENABLED": "1", "LOGGING_LOG_BODIES": "false"},
			check: func(t *testing.T, cfg *Config) {
				if !cfg.Metrics.Enabled {
					t.Error("Metrics.Enabled should be true")
				}
				if !cfg.Logging.Enabled {
					t.Error("Logging.Enabled should be true")
				}
				if cfg.Logging.LogBodies {
					t.Error("Logging.LogBodies should be false")
				}
			},
		},
		{
			name:    "HTTP timeout overrides",
			envVars: map[string]string{"HTTP_TIMEOUT": "30", "HTTP_RESPONSE_HEADER_TIMEOUT": "60"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.HTTP.Timeout != 30 {
					t.Errorf("HTTP.Timeout = %d, want 30", cfg.HTTP.Timeout)
				}
				if cfg.HTTP.ResponseHeaderTimeout != 60 {
					t.Errorf("HTTP.ResponseHeaderTimeout = %d, want 60", cfg.HTTP.ResponseHeaderTimeout)
				}
			},
		},
		{
			name:    "CACHE_REFRESH_INTERVAL override",
			envVars: map[string]string{"CACHE_REFRESH_INTERVAL": "1800"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.Cache.RefreshInterval != 1800 {
					t.Errorf("Cache.RefreshInterval = %d, want 1800", cfg.Cache.RefreshInterval)
				}
			},
		},
		{
			name:    "no env vars set preserves defaults",
			envVars: map[string]string{},
			check: func(t *testing.T, cfg *Config) {
				if cfg.Server.Port != "8080" {
					t.Errorf("Server.Port = %q, want %q", cfg.Server.Port, "8080")
				}
				if cfg.HTTP.Timeout != 600 {
					t.Errorf("HTTP.Timeout = %d, want 600", cfg.HTTP.Timeout)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			cfg := buildDefaultConfig()
			require.NoError(t, applyEnvOverrides(cfg))
			tt.check(t, cfg)
		})
	}
}

// TestApplyProviderEnvVars tests the applyProviderEnvVars function
func TestApplyProviderEnvVars(t *testing.T) {
	t.Run("discovers provider from API key", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "sk-test-123")
		cfg := buildDefaultConfig()
		rawProviders := make(map[string]rawProviderConfig)
		applyProviderEnvVars(cfg, rawProviders)

		p, ok := rawProviders["openai"]
		if !ok {
			t.Fatal("expected openai provider")
		}
		if p.APIKey != "sk-test-123" {
			t.Errorf("APIKey = %q, want %q", p.APIKey, "sk-test-123")
		}
		if p.Type != "openai" {
			t.Errorf("Type = %q, want %q", p.Type, "openai")
		}
	})

	t.Run("overrides existing YAML provider", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "sk-env-key")
		cfg := buildDefaultConfig()
		rawProviders := map[string]rawProviderConfig{
			"openai": {Type: "openai", APIKey: "sk-yaml-key", BaseURL: "https://custom.api.com"},
		}
		applyProviderEnvVars(cfg, rawProviders)

		p := rawProviders["openai"]
		if p.APIKey != "sk-env-key" {
			t.Errorf("APIKey = %q, want %q (env should override yaml)", p.APIKey, "sk-env-key")
		}
		if p.BaseURL != "https://custom.api.com" {
			t.Errorf("BaseURL = %q should be preserved from YAML", p.BaseURL)
		}
	})

	t.Run("ollama enabled via base URL only", func(t *testing.T) {
		t.Setenv("OLLAMA_BASE_URL", "http://localhost:11434/v1")
		cfg := buildDefaultConfig()
		rawProviders := make(map[string]rawProviderConfig)
		applyProviderEnvVars(cfg, rawProviders)

		p, ok := rawProviders["ollama"]
		if !ok {
			t.Fatal("expected ollama provider")
		}
		if p.BaseURL != "http://localhost:11434/v1" {
			t.Errorf("BaseURL = %q", p.BaseURL)
		}
	})

	t.Run("skips when no env vars set", func(t *testing.T) {
		cfg := buildDefaultConfig()
		rawProviders := make(map[string]rawProviderConfig)
		applyProviderEnvVars(cfg, rawProviders)

		if len(rawProviders) != 0 {
			t.Errorf("expected no providers, got %d", len(rawProviders))
		}
	})
}

// TestIntegration_ExpandAndFilter tests env var expansion in YAML + provider filtering
func TestIntegration_ExpandAndFilter(t *testing.T) {
	t.Run("expand and filter mixed providers", func(t *testing.T) {
		_ = os.Setenv("OPENAI_API_KEY", "sk-openai-123")
		defer os.Unsetenv("OPENAI_API_KEY")

		rawProviders := map[string]rawProviderConfig{
			"openai": {
				Type:   "openai",
				APIKey: "sk-openai-123",
			},
			"openai-fallback": {
				Type:   "openai",
				APIKey: "${OPENAI_FALLBACK_KEY}",
			},
			"anthropic": {
				Type:   "anthropic",
				APIKey: "${ANTHROPIC_API_KEY}",
			},
		}
		removeEmptyProviders(rawProviders)

		if len(rawProviders) != 1 {
			t.Errorf("expected 1 provider, got %d: %v", len(rawProviders), rawProviders)
		}
		if _, ok := rawProviders["openai"]; !ok {
			t.Error("expected openai to survive filtering")
		}
	})
}
