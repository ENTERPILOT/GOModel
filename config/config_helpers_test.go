package config

import (
	"os"
	"testing"
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

// TestExpandEnvVars tests the expandEnvVars function
func TestExpandEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		input    Config
		envVars  map[string]string
		expected Config
	}{
		{
			name: "expand server port",
			input: Config{
				Server: ServerConfig{
					Port: "${PORT}",
				},
				Providers: map[string]ProviderConfig{},
			},
			envVars: map[string]string{"PORT": "3000"},
			expected: Config{
				Server: ServerConfig{
					Port: "3000",
				},
				Providers: map[string]ProviderConfig{},
			},
		},
		{
			name: "expand provider API key",
			input: Config{
				Server: ServerConfig{
					Port: "8080",
				},
				Providers: map[string]ProviderConfig{
					"openai": {
						Type:   "openai",
						APIKey: "${OPENAI_API_KEY}",
					},
				},
			},
			envVars: map[string]string{"OPENAI_API_KEY": "sk-test-123"},
			expected: Config{
				Server: ServerConfig{
					Port: "8080",
				},
				Providers: map[string]ProviderConfig{
					"openai": {
						Type:   "openai",
						APIKey: "sk-test-123",
					},
				},
			},
		},
		{
			name: "expand provider base URL",
			input: Config{
				Server: ServerConfig{
					Port: "8080",
				},
				Providers: map[string]ProviderConfig{
					"openai": {
						Type:    "openai",
						APIKey:  "sk-test-123",
						BaseURL: "${OPENAI_BASE_URL}",
					},
				},
			},
			envVars: map[string]string{"OPENAI_BASE_URL": "https://custom.api.com"},
			expected: Config{
				Server: ServerConfig{
					Port: "8080",
				},
				Providers: map[string]ProviderConfig{
					"openai": {
						Type:    "openai",
						APIKey:  "sk-test-123",
						BaseURL: "https://custom.api.com",
					},
				},
			},
		},
		{
			name: "multiple providers with mixed expansion",
			input: Config{
				Server: ServerConfig{
					Port: "${PORT:-8080}",
				},
				Providers: map[string]ProviderConfig{
					"openai": {
						Type:   "openai",
						APIKey: "${OPENAI_API_KEY}",
					},
					"anthropic": {
						Type:   "anthropic",
						APIKey: "${ANTHROPIC_API_KEY}",
					},
					"gemini": {
						Type:   "gemini",
						APIKey: "${GEMINI_API_KEY}",
					},
				},
			},
			envVars: map[string]string{
				"OPENAI_API_KEY":    "sk-openai-123",
				"ANTHROPIC_API_KEY": "sk-ant-456",
				// GEMINI_API_KEY intentionally missing
			},
			expected: Config{
				Server: ServerConfig{
					Port: "8080",
				},
				Providers: map[string]ProviderConfig{
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
						APIKey: "${GEMINI_API_KEY}",
					},
				},
			},
		},
		{
			name: "unresolved variables remain as placeholders",
			input: Config{
				Server: ServerConfig{
					Port: "${MISSING_PORT}",
				},
				Providers: map[string]ProviderConfig{
					"openai": {
						Type:    "openai",
						APIKey:  "${MISSING_KEY}",
						BaseURL: "${MISSING_URL}",
					},
				},
			},
			envVars: map[string]string{},
			expected: Config{
				Server: ServerConfig{
					Port: "${MISSING_PORT}",
				},
				Providers: map[string]ProviderConfig{
					"openai": {
						Type:    "openai",
						APIKey:  "${MISSING_KEY}",
						BaseURL: "${MISSING_URL}",
					},
				},
			},
		},
		{
			name: "empty config",
			input: Config{
				Server:    ServerConfig{},
				Providers: map[string]ProviderConfig{},
			},
			envVars: map[string]string{},
			expected: Config{
				Server:    ServerConfig{},
				Providers: map[string]ProviderConfig{},
			},
		},
		{
			name: "config with default values in placeholders",
			input: Config{
				Server: ServerConfig{
					Port: "${PORT:-9000}",
				},
				Providers: map[string]ProviderConfig{
					"openai": {
						Type:    "openai",
						APIKey:  "${OPENAI_API_KEY}",
						BaseURL: "${OPENAI_BASE_URL:-https://api.openai.com}",
					},
				},
			},
			envVars: map[string]string{
				"OPENAI_API_KEY": "sk-test-789",
			},
			expected: Config{
				Server: ServerConfig{
					Port: "9000",
				},
				Providers: map[string]ProviderConfig{
					"openai": {
						Type:    "openai",
						APIKey:  "sk-test-789",
						BaseURL: "https://api.openai.com",
					},
				},
			},
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

			result := expandEnvVars(tt.input)

			// Compare server config
			if result.Server.Port != tt.expected.Server.Port {
				t.Errorf("Server.Port = %q, want %q", result.Server.Port, tt.expected.Server.Port)
			}

			// Compare providers
			if len(result.Providers) != len(tt.expected.Providers) {
				t.Errorf("len(Providers) = %d, want %d", len(result.Providers), len(tt.expected.Providers))
			}

			for name, expectedProvider := range tt.expected.Providers {
				resultProvider, exists := result.Providers[name]
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
		})
	}
}

// TestRemoveEmptyProviders tests the removeEmptyProviders function
func TestRemoveEmptyProviders(t *testing.T) {
	tests := []struct {
		name     string
		input    Config
		expected Config
	}{
		{
			name: "remove provider with empty API key",
			input: Config{
				Server: ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{
					"openai": {
						Type:   "openai",
						APIKey: "",
					},
					"anthropic": {
						Type:   "anthropic",
						APIKey: "sk-ant-valid",
					},
				},
			},
			expected: Config{
				Server: ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{
					"anthropic": {
						Type:   "anthropic",
						APIKey: "sk-ant-valid",
					},
				},
			},
		},
		{
			name: "remove provider with unresolved placeholder",
			input: Config{
				Server: ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{
					"openai": {
						Type:   "openai",
						APIKey: "${OPENAI_API_KEY}",
					},
					"anthropic": {
						Type:   "anthropic",
						APIKey: "sk-ant-valid",
					},
				},
			},
			expected: Config{
				Server: ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{
					"anthropic": {
						Type:   "anthropic",
						APIKey: "sk-ant-valid",
					},
				},
			},
		},
		{
			name: "remove provider with partially resolved placeholder",
			input: Config{
				Server: ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{
					"openai": {
						Type:   "openai",
						APIKey: "prefix-${UNRESOLVED}",
					},
					"anthropic": {
						Type:   "anthropic",
						APIKey: "sk-ant-valid",
					},
				},
			},
			expected: Config{
				Server: ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{
					"anthropic": {
						Type:   "anthropic",
						APIKey: "sk-ant-valid",
					},
				},
			},
		},
		{
			name: "keep all providers with valid API keys",
			input: Config{
				Server: ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{
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
			expected: Config{
				Server: ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{
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
		},
		{
			name: "remove all providers when all have invalid keys",
			input: Config{
				Server: ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{
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
			},
			expected: Config{
				Server:    ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{},
			},
		},
		{
			name: "mixed valid and invalid providers",
			input: Config{
				Server: ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{
					"openai-primary": {
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
			},
			expected: Config{
				Server: ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{
					"openai-primary": {
						Type:   "openai",
						APIKey: "sk-openai-valid",
					},
					"gemini": {
						Type:   "gemini",
						APIKey: "sk-gemini-valid",
					},
				},
			},
		},
		{
			name: "empty providers map",
			input: Config{
				Server:    ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{},
			},
			expected: Config{
				Server:    ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{},
			},
		},
		{
			name: "provider with valid API key but empty BaseURL should be kept",
			input: Config{
				Server: ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{
					"openai": {
						Type:    "openai",
						APIKey:  "sk-openai-123",
						BaseURL: "",
					},
				},
			},
			expected: Config{
				Server: ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{
					"openai": {
						Type:    "openai",
						APIKey:  "sk-openai-123",
						BaseURL: "",
					},
				},
			},
		},
		{
			name: "provider with valid API key but unresolved BaseURL should be kept",
			input: Config{
				Server: ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{
					"openai": {
						Type:    "openai",
						APIKey:  "sk-openai-123",
						BaseURL: "${CUSTOM_URL}",
					},
				},
			},
			expected: Config{
				Server: ServerConfig{Port: "8080"},
				Providers: map[string]ProviderConfig{
					"openai": {
						Type:    "openai",
						APIKey:  "sk-openai-123",
						BaseURL: "${CUSTOM_URL}",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeEmptyProviders(tt.input)

			// Compare server config (should remain unchanged)
			if result.Server.Port != tt.expected.Server.Port {
				t.Errorf("Server.Port = %q, want %q", result.Server.Port, tt.expected.Server.Port)
			}

			// Compare number of providers
			if len(result.Providers) != len(tt.expected.Providers) {
				t.Errorf("len(Providers) = %d, want %d", len(result.Providers), len(tt.expected.Providers))
			}

			// Check each expected provider exists with correct values
			for name, expectedProvider := range tt.expected.Providers {
				resultProvider, exists := result.Providers[name]
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

			// Check that no unexpected providers exist in result
			for name := range result.Providers {
				if _, exists := tt.expected.Providers[name]; !exists {
					t.Errorf("Unexpected provider %q found in result", name)
				}
			}
		})
	}
}

// TestExpandEnvVars_MasterKey specifically tests master key expansion to prevent auth bypass bugs
func TestExpandEnvVars_MasterKey(t *testing.T) {
	tests := []struct {
		name              string
		input             Config
		envVars           map[string]string
		expectedMasterKey string
	}{
		{
			name: "master key not set with empty default should be empty string",
			input: Config{
				Server: ServerConfig{
					Port:      "8080",
					MasterKey: "${GOMODEL_MASTER_KEY:-}",
				},
				Providers: map[string]ProviderConfig{},
			},
			envVars:           map[string]string{},
			expectedMasterKey: "",
		},
		{
			name: "master key set should use the value",
			input: Config{
				Server: ServerConfig{
					Port:      "8080",
					MasterKey: "${GOMODEL_MASTER_KEY:-}",
				},
				Providers: map[string]ProviderConfig{},
			},
			envVars:           map[string]string{"GOMODEL_MASTER_KEY": "my-secret-key"},
			expectedMasterKey: "my-secret-key",
		},
		{
			name: "master key with non-empty default - not set should use default",
			input: Config{
				Server: ServerConfig{
					Port:      "8080",
					MasterKey: "${GOMODEL_MASTER_KEY:-default-secret}",
				},
				Providers: map[string]ProviderConfig{},
			},
			envVars:           map[string]string{},
			expectedMasterKey: "default-secret",
		},
		{
			name: "master key without default syntax - not set should keep placeholder",
			input: Config{
				Server: ServerConfig{
					Port:      "8080",
					MasterKey: "${GOMODEL_MASTER_KEY}",
				},
				Providers: map[string]ProviderConfig{},
			},
			envVars:           map[string]string{},
			expectedMasterKey: "${GOMODEL_MASTER_KEY}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing env var first
			_ = os.Unsetenv("GOMODEL_MASTER_KEY")

			// Setup environment variables
			for k, v := range tt.envVars {
				_ = os.Setenv(k, v)
			}
			// Cleanup after test
			defer func() {
				_ = os.Unsetenv("GOMODEL_MASTER_KEY")
				for k := range tt.envVars {
					_ = os.Unsetenv(k)
				}
			}()

			result := expandEnvVars(tt.input)

			if result.Server.MasterKey != tt.expectedMasterKey {
				t.Errorf("Server.MasterKey = %q, want %q", result.Server.MasterKey, tt.expectedMasterKey)
			}
		})
	}
}

// TestIntegration_ExpandAndFilter tests the combination of expandEnvVars and removeEmptyProviders
func TestIntegration_ExpandAndFilter(t *testing.T) {
	tests := []struct {
		name     string
		input    Config
		envVars  map[string]string
		expected Config
	}{
		{
			name: "expand and filter mixed providers",
			input: Config{
				Server: ServerConfig{
					Port: "${PORT:-8080}",
				},
				Providers: map[string]ProviderConfig{
					"openai-primary": {
						Type:   "openai",
						APIKey: "${OPENAI_API_KEY}",
					},
					"openai-fallback": {
						Type:   "openai",
						APIKey: "${OPENAI_FALLBACK_KEY}",
					},
					"anthropic": {
						Type:   "anthropic",
						APIKey: "${ANTHROPIC_API_KEY}",
					},
				},
			},
			envVars: map[string]string{
				"OPENAI_API_KEY": "sk-openai-123",
				// OPENAI_FALLBACK_KEY and ANTHROPIC_API_KEY intentionally missing
			},
			expected: Config{
				Server: ServerConfig{
					Port: "8080",
				},
				Providers: map[string]ProviderConfig{
					"openai-primary": {
						Type:   "openai",
						APIKey: "sk-openai-123",
					},
				},
			},
		},
		{
			name: "all providers filtered when none have valid keys",
			input: Config{
				Server: ServerConfig{
					Port: "8080",
				},
				Providers: map[string]ProviderConfig{
					"openai": {
						Type:   "openai",
						APIKey: "${OPENAI_API_KEY}",
					},
					"anthropic": {
						Type:   "anthropic",
						APIKey: "${ANTHROPIC_API_KEY}",
					},
				},
			},
			envVars: map[string]string{},
			expected: Config{
				Server: ServerConfig{
					Port: "8080",
				},
				Providers: map[string]ProviderConfig{},
			},
		},
		{
			name: "complex scenario with defaults and partial resolution",
			input: Config{
				Server: ServerConfig{
					Port: "${PORT:-9000}",
				},
				Providers: map[string]ProviderConfig{
					"provider1": {
						Type:    "openai",
						APIKey:  "${API_KEY_1}",
						BaseURL: "${BASE_URL_1:-https://api.default1.com}",
					},
					"provider2": {
						Type:    "openai",
						APIKey:  "${API_KEY_2:-default-key}",
						BaseURL: "${BASE_URL_2}",
					},
					"provider3": {
						Type:    "anthropic",
						APIKey:  "${API_KEY_3}",
						BaseURL: "",
					},
				},
			},
			envVars: map[string]string{
				"API_KEY_1": "sk-valid-1",
				// API_KEY_2 will use default
				// API_KEY_3 is missing (no default)
			},
			expected: Config{
				Server: ServerConfig{
					Port: "9000",
				},
				Providers: map[string]ProviderConfig{
					"provider1": {
						Type:    "openai",
						APIKey:  "sk-valid-1",
						BaseURL: "https://api.default1.com",
					},
					"provider2": {
						Type:    "openai",
						APIKey:  "default-key",
						BaseURL: "${BASE_URL_2}",
					},
				},
			},
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

			// Apply both functions in sequence (as done in Load())
			result := expandEnvVars(tt.input)
			result = removeEmptyProviders(result)

			// Compare server config
			if result.Server.Port != tt.expected.Server.Port {
				t.Errorf("Server.Port = %q, want %q", result.Server.Port, tt.expected.Server.Port)
			}

			// Compare providers
			if len(result.Providers) != len(tt.expected.Providers) {
				t.Errorf("len(Providers) = %d, want %d", len(result.Providers), len(tt.expected.Providers))
			}

			for name, expectedProvider := range tt.expected.Providers {
				resultProvider, exists := result.Providers[name]
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
		})
	}
}
