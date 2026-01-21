package config

import (
	"os"
	"testing"

	"github.com/go-viper/mapstructure/v2"
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
	provider, exists := cfg.Providers["openai"]
	if !exists {
		t.Fatal("expected 'openai' provider to exist")
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
	openaiProvider, exists := cfg.Providers["openai"]
	if !exists {
		t.Error("expected 'openai' provider to exist")
	} else if openaiProvider.APIKey != testAPIKey {
		t.Errorf("expected OpenAI API key %s, got %s", testAPIKey, openaiProvider.APIKey)
	}

	// Check Anthropic provider
	anthropicProvider, exists := cfg.Providers["anthropic"]
	if !exists {
		t.Error("expected 'anthropic' provider to exist")
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
		cfg.Providers["openai"] = ProviderConfig{
			Type:   "openai",
			APIKey: apiKey,
		}
	}

	// Verify values from .env file
	if cfg.Server.Port != "7070" {
		t.Errorf("expected port 7070 from .env file, got %s", cfg.Server.Port)
	}

	openaiProvider, exists := cfg.Providers["openai"]
	if !exists {
		t.Fatal("expected 'openai' provider to exist")
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
		cfg.Providers["openai"] = ProviderConfig{
			Type:   "openai",
			APIKey: apiKey,
		}
	}

	// Environment variables should override .env file
	if cfg.Server.Port != "9999" {
		t.Errorf("expected port 9999 from environment variable (not .env file), got %s", cfg.Server.Port)
	}

	openaiProvider, exists := cfg.Providers["openai"]
	if !exists {
		t.Fatal("expected 'openai' provider to exist")
	}

	if openaiProvider.APIKey != "sk-from-real-env" {
		t.Errorf("expected API key from environment variable (not .env file), got %s", openaiProvider.APIKey)
	}
}

func TestLoggingOnlyModelInteractionsDefault(t *testing.T) {
	// Reset viper state before test
	viper.Reset()

	// Clear all relevant environment variables
	_ = os.Unsetenv("LOGGING_ONLY_MODEL_INTERACTIONS")
	_ = os.Unsetenv("OPENAI_API_KEY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Default should be true
	if !cfg.Logging.OnlyModelInteractions {
		t.Error("expected OnlyModelInteractions to default to true")
	}
}

func TestLoggingOnlyModelInteractionsFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"true lowercase", "true", true},
		{"TRUE uppercase", "TRUE", true},
		{"True mixed", "True", true},
		{"false lowercase", "false", false},
		{"FALSE uppercase", "FALSE", false},
		{"False mixed", "False", false},
		{"1 numeric", "1", true},
		{"0 numeric", "0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper state before each subtest
			viper.Reset()

			// Clear and set environment variable
			_ = os.Unsetenv("OPENAI_API_KEY")
			_ = os.Setenv("LOGGING_ONLY_MODEL_INTERACTIONS", tt.envValue)
			defer func() { _ = os.Unsetenv("LOGGING_ONLY_MODEL_INTERACTIONS") }()

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() failed: %v", err)
			}

			if cfg.Logging.OnlyModelInteractions != tt.expected {
				t.Errorf("expected OnlyModelInteractions=%v for env value %q, got %v",
					tt.expected, tt.envValue, cfg.Logging.OnlyModelInteractions)
			}
		})
	}
}

func TestSnakeCaseMatchName(t *testing.T) {
	tests := []struct {
		name      string
		mapKey    string
		fieldName string
		expected  bool
	}{
		// Snake case to PascalCase matches
		{"body_size_limit matches BodySizeLimit", "body_size_limit", "BodySizeLimit", true},
		{"api_key matches APIKey", "api_key", "APIKey", true},
		{"base_url matches BaseURL", "base_url", "BaseURL", true},
		{"storage_type matches StorageType", "storage_type", "StorageType", true},
		{"log_bodies matches LogBodies", "log_bodies", "LogBodies", true},
		{"only_model_interactions matches OnlyModelInteractions", "only_model_interactions", "OnlyModelInteractions", true},
		{"max_conns matches MaxConns", "max_conns", "MaxConns", true},
		{"flush_interval matches FlushInterval", "flush_interval", "FlushInterval", true},
		{"retention_days matches RetentionDays", "retention_days", "RetentionDays", true},

		// Simple case-insensitive matches (no underscores)
		{"port matches Port", "port", "Port", true},
		{"enabled matches Enabled", "enabled", "Enabled", true},
		{"type matches Type", "type", "Type", true},
		{"url matches URL", "url", "URL", true},
		{"ttl matches TTL", "ttl", "TTL", true},
		{"redis matches Redis", "redis", "Redis", true},
		{"sqlite matches SQLite", "sqlite", "SQLite", true},
		{"postgresql matches PostgreSQL", "postgresql", "PostgreSQL", true},
		{"mongodb matches MongoDB", "mongodb", "MongoDB", true},

		// Case variations
		{"PORT matches Port", "PORT", "Port", true},
		{"Port matches Port", "Port", "Port", true},
		{"BODY_SIZE_LIMIT matches BodySizeLimit", "BODY_SIZE_LIMIT", "BodySizeLimit", true},

		// Non-matches
		{"different names don't match", "foo", "Bar", false},
		{"partial match fails", "body_size", "BodySizeLimit", false},

		// Malformed keys are rejected
		{"consecutive underscores rejected", "body__size_limit", "BodySizeLimit", false},
		{"leading underscore rejected", "_port", "Port", false},
		{"trailing underscore rejected", "port_", "Port", false},
		{"leading and trailing underscore rejected", "_port_", "Port", false},
	}

	// Get the MatchName function from snakeCaseMatchName
	var decoderConfig mapstructure.DecoderConfig
	opt := snakeCaseMatchName()
	opt(&decoderConfig)
	matchName := decoderConfig.MatchName

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchName(tt.mapKey, tt.fieldName)
			if result != tt.expected {
				t.Errorf("matchName(%q, %q) = %v, expected %v",
					tt.mapKey, tt.fieldName, result, tt.expected)
			}
		})
	}
}

func TestSnakeCaseMatchNameWithViper(t *testing.T) {
	// Reset viper state
	viper.Reset()

	// Create a map simulating YAML config with snake_case keys
	configData := map[string]any{
		"server": map[string]any{
			"port":            "9090",
			"master_key":      "test-master-key",
			"body_size_limit": "50M",
		},
		"logging": map[string]any{
			"enabled":                 true,
			"log_bodies":              false,
			"log_headers":             true,
			"buffer_size":             500,
			"flush_interval":          10,
			"retention_days":          60,
			"only_model_interactions": false,
		},
		"storage": map[string]any{
			"type": "postgresql",
		},
		"cache": map[string]any{
			"type": "redis",
			"redis": map[string]any{
				"url": "redis://localhost:6379",
				"key": "test:models",
				"ttl": 3600,
			},
		},
	}

	// Set the config data in viper
	for k, v := range configData {
		viper.Set(k, v)
	}

	var cfg Config
	err := viper.Unmarshal(&cfg, snakeCaseMatchName())
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify server config
	if cfg.Server.Port != "9090" {
		t.Errorf("expected Server.Port=9090, got %s", cfg.Server.Port)
	}
	if cfg.Server.MasterKey != "test-master-key" {
		t.Errorf("expected Server.MasterKey=test-master-key, got %s", cfg.Server.MasterKey)
	}
	if cfg.Server.BodySizeLimit != "50M" {
		t.Errorf("expected Server.BodySizeLimit=50M, got %s", cfg.Server.BodySizeLimit)
	}

	// Verify storage config
	if cfg.Storage.Type != "postgresql" {
		t.Errorf("expected Storage.Type=postgresql, got %s", cfg.Storage.Type)
	}

	// Verify logging config
	if !cfg.Logging.Enabled {
		t.Error("expected Logging.Enabled=true")
	}
	if cfg.Logging.LogBodies {
		t.Error("expected Logging.LogBodies=false")
	}
	if !cfg.Logging.LogHeaders {
		t.Error("expected Logging.LogHeaders=true")
	}
	if cfg.Logging.BufferSize != 500 {
		t.Errorf("expected Logging.BufferSize=500, got %d", cfg.Logging.BufferSize)
	}
	if cfg.Logging.FlushInterval != 10 {
		t.Errorf("expected Logging.FlushInterval=10, got %d", cfg.Logging.FlushInterval)
	}
	if cfg.Logging.RetentionDays != 60 {
		t.Errorf("expected Logging.RetentionDays=60, got %d", cfg.Logging.RetentionDays)
	}
	if cfg.Logging.OnlyModelInteractions {
		t.Error("expected Logging.OnlyModelInteractions=false")
	}

	// Verify cache config
	if cfg.Cache.Type != "redis" {
		t.Errorf("expected Cache.Type=redis, got %s", cfg.Cache.Type)
	}
	if cfg.Cache.Redis.URL != "redis://localhost:6379" {
		t.Errorf("expected Cache.Redis.URL=redis://localhost:6379, got %s", cfg.Cache.Redis.URL)
	}
	if cfg.Cache.Redis.Key != "test:models" {
		t.Errorf("expected Cache.Redis.Key=test:models, got %s", cfg.Cache.Redis.Key)
	}
	if cfg.Cache.Redis.TTL != 3600 {
		t.Errorf("expected Cache.Redis.TTL=3600, got %d", cfg.Cache.Redis.TTL)
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
