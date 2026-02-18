package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// clearProviderEnvVars unsets all known provider-related environment variables.
func clearProviderEnvVars(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"OPENAI_API_KEY", "OPENAI_BASE_URL",
		"ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL",
		"GEMINI_API_KEY", "GEMINI_BASE_URL",
		"XAI_API_KEY", "XAI_BASE_URL",
		"GROQ_API_KEY", "GROQ_BASE_URL",
		"OLLAMA_API_KEY", "OLLAMA_BASE_URL",
	} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
}

// clearAllConfigEnvVars unsets all config-related environment variables.
func clearAllConfigEnvVars(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"PORT", "GOMODEL_MASTER_KEY", "BODY_SIZE_LIMIT",
		"CACHE_TYPE", "GOMODEL_CACHE_DIR",
		"REDIS_URL", "REDIS_KEY", "REDIS_TTL",
		"STORAGE_TYPE", "SQLITE_PATH", "POSTGRES_URL", "POSTGRES_MAX_CONNS",
		"MONGODB_URL", "MONGODB_DATABASE",
		"METRICS_ENABLED", "METRICS_ENDPOINT",
		"LOGGING_ENABLED", "LOGGING_LOG_BODIES", "LOGGING_LOG_HEADERS",
		"LOGGING_ONLY_MODEL_INTERACTIONS", "LOGGING_BUFFER_SIZE",
		"LOGGING_FLUSH_INTERVAL", "LOGGING_RETENTION_DAYS",
		"USAGE_ENABLED", "ENFORCE_RETURNING_USAGE_DATA",
		"USAGE_BUFFER_SIZE", "USAGE_FLUSH_INTERVAL", "USAGE_RETENTION_DAYS",
		"HTTP_TIMEOUT", "HTTP_RESPONSE_HEADER_TIMEOUT",
	} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
	clearProviderEnvVars(t)
}

// withTempDir runs fn in a temporary directory, restoring the original working directory afterward.
func withTempDir(t *testing.T, fn func(dir string)) {
	t.Helper()
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	fn(tempDir)
}

func TestBuildDefaultConfig(t *testing.T) {
	cfg := buildDefaultConfig()

	if cfg.Server.Port != "8080" {
		t.Errorf("expected Server.Port=8080, got %s", cfg.Server.Port)
	}
	if cfg.Cache.Type != "local" {
		t.Errorf("expected Cache.Type=local, got %s", cfg.Cache.Type)
	}
	if cfg.Cache.CacheDir != ".cache" {
		t.Errorf("expected Cache.CacheDir=.cache, got %s", cfg.Cache.CacheDir)
	}
	if cfg.Cache.RefreshInterval != 3600 {
		t.Errorf("expected Cache.RefreshInterval=3600, got %d", cfg.Cache.RefreshInterval)
	}
	if cfg.Cache.Redis.Key != "gomodel:models" {
		t.Errorf("expected Cache.Redis.Key=gomodel:models, got %s", cfg.Cache.Redis.Key)
	}
	if cfg.Cache.Redis.TTL != 86400 {
		t.Errorf("expected Cache.Redis.TTL=86400, got %d", cfg.Cache.Redis.TTL)
	}
	if cfg.Storage.Type != "sqlite" {
		t.Errorf("expected Storage.Type=sqlite, got %s", cfg.Storage.Type)
	}
	if cfg.Storage.SQLite.Path != "data/gomodel.db" {
		t.Errorf("expected Storage.SQLite.Path=data/gomodel.db, got %s", cfg.Storage.SQLite.Path)
	}
	if cfg.Storage.PostgreSQL.MaxConns != 10 {
		t.Errorf("expected Storage.PostgreSQL.MaxConns=10, got %d", cfg.Storage.PostgreSQL.MaxConns)
	}
	if cfg.Storage.MongoDB.Database != "gomodel" {
		t.Errorf("expected Storage.MongoDB.Database=gomodel, got %s", cfg.Storage.MongoDB.Database)
	}
	if !cfg.Logging.LogBodies {
		t.Error("expected Logging.LogBodies=true")
	}
	if !cfg.Logging.LogHeaders {
		t.Error("expected Logging.LogHeaders=true")
	}
	if cfg.Logging.BufferSize != 1000 {
		t.Errorf("expected Logging.BufferSize=1000, got %d", cfg.Logging.BufferSize)
	}
	if cfg.Logging.FlushInterval != 5 {
		t.Errorf("expected Logging.FlushInterval=5, got %d", cfg.Logging.FlushInterval)
	}
	if cfg.Logging.RetentionDays != 30 {
		t.Errorf("expected Logging.RetentionDays=30, got %d", cfg.Logging.RetentionDays)
	}
	if !cfg.Logging.OnlyModelInteractions {
		t.Error("expected Logging.OnlyModelInteractions=true")
	}
	if cfg.Logging.Enabled {
		t.Error("expected Logging.Enabled=false")
	}
	if !cfg.Usage.Enabled {
		t.Error("expected Usage.Enabled=true")
	}
	if !cfg.Usage.EnforceReturningUsageData {
		t.Error("expected Usage.EnforceReturningUsageData=true")
	}
	if cfg.Usage.BufferSize != 1000 {
		t.Errorf("expected Usage.BufferSize=1000, got %d", cfg.Usage.BufferSize)
	}
	if cfg.Usage.FlushInterval != 5 {
		t.Errorf("expected Usage.FlushInterval=5, got %d", cfg.Usage.FlushInterval)
	}
	if cfg.Usage.RetentionDays != 90 {
		t.Errorf("expected Usage.RetentionDays=90, got %d", cfg.Usage.RetentionDays)
	}
	if cfg.Metrics.Endpoint != "/metrics" {
		t.Errorf("expected Metrics.Endpoint=/metrics, got %s", cfg.Metrics.Endpoint)
	}
	if cfg.Metrics.Enabled {
		t.Error("expected Metrics.Enabled=false")
	}
	if cfg.HTTP.Timeout != 600 {
		t.Errorf("expected HTTP.Timeout=600, got %d", cfg.HTTP.Timeout)
	}
	if cfg.HTTP.ResponseHeaderTimeout != 600 {
		t.Errorf("expected HTTP.ResponseHeaderTimeout=600, got %d", cfg.HTTP.ResponseHeaderTimeout)
	}
	if cfg.Providers == nil {
		t.Error("expected Providers to be initialized (non-nil)")
	}

	expected := DefaultRetryConfig()
	if cfg.Resilience.Retry != expected {
		t.Errorf("expected Resilience.Retry=%+v, got %+v", expected, cfg.Resilience.Retry)
	}
}

func TestBuildProviderConfig_InheritsGlobalDefaults(t *testing.T) {
	global := ResilienceConfig{
		Retry: RetryConfig{
			MaxRetries:     5,
			InitialBackoff: 2 * time.Second,
			MaxBackoff:     60 * time.Second,
			BackoffFactor:  3.0,
			JitterFactor:   0.2,
		},
	}
	input := rawProviderConfig{
		Type:   "openai",
		APIKey: "sk-test",
	}

	result := buildProviderConfig(input, global)

	if result.Type != "openai" {
		t.Errorf("expected Type=openai, got %s", result.Type)
	}
	if result.Resilience.Retry != global.Retry {
		t.Errorf("expected provider to inherit global retry config\ngot:  %+v\nwant: %+v", result.Resilience.Retry, global.Retry)
	}
}

func TestBuildProviderConfig_OverridesSpecificFields(t *testing.T) {
	global := ResilienceConfig{
		Retry: DefaultRetryConfig(),
	}
	maxRetries := 10
	jitter := 0.5
	input := rawProviderConfig{
		Type:   "anthropic",
		APIKey: "sk-ant",
		Resilience: &rawResilienceConfig{
			Retry: &rawRetryConfig{
				MaxRetries:   &maxRetries,
				JitterFactor: &jitter,
			},
		},
	}

	result := buildProviderConfig(input, global)

	if result.Resilience.Retry.MaxRetries != 10 {
		t.Errorf("expected MaxRetries=10, got %d", result.Resilience.Retry.MaxRetries)
	}
	if result.Resilience.Retry.JitterFactor != 0.5 {
		t.Errorf("expected JitterFactor=0.5, got %f", result.Resilience.Retry.JitterFactor)
	}
	if result.Resilience.Retry.InitialBackoff != global.Retry.InitialBackoff {
		t.Errorf("expected InitialBackoff to be inherited from global, got %v", result.Resilience.Retry.InitialBackoff)
	}
	if result.Resilience.Retry.MaxBackoff != global.Retry.MaxBackoff {
		t.Errorf("expected MaxBackoff to be inherited from global, got %v", result.Resilience.Retry.MaxBackoff)
	}
	if result.Resilience.Retry.BackoffFactor != global.Retry.BackoffFactor {
		t.Errorf("expected BackoffFactor to be inherited from global, got %f", result.Resilience.Retry.BackoffFactor)
	}
}

func TestBuildProviderConfig_OverridesAllFields(t *testing.T) {
	global := ResilienceConfig{Retry: DefaultRetryConfig()}
	maxRetries := 7
	initial := 500 * time.Millisecond
	maxBack := 10 * time.Second
	factor := 1.5
	jitter := 0.3
	input := rawProviderConfig{
		Type:   "gemini",
		APIKey: "sk-gem",
		Resilience: &rawResilienceConfig{
			Retry: &rawRetryConfig{
				MaxRetries:     &maxRetries,
				InitialBackoff: &initial,
				MaxBackoff:     &maxBack,
				BackoffFactor:  &factor,
				JitterFactor:   &jitter,
			},
		},
	}

	result := buildProviderConfig(input, global)

	if result.Resilience.Retry.MaxRetries != 7 {
		t.Errorf("expected MaxRetries=7, got %d", result.Resilience.Retry.MaxRetries)
	}
	if result.Resilience.Retry.InitialBackoff != 500*time.Millisecond {
		t.Errorf("expected InitialBackoff=500ms, got %v", result.Resilience.Retry.InitialBackoff)
	}
	if result.Resilience.Retry.MaxBackoff != 10*time.Second {
		t.Errorf("expected MaxBackoff=10s, got %v", result.Resilience.Retry.MaxBackoff)
	}
	if result.Resilience.Retry.BackoffFactor != 1.5 {
		t.Errorf("expected BackoffFactor=1.5, got %f", result.Resilience.Retry.BackoffFactor)
	}
	if result.Resilience.Retry.JitterFactor != 0.3 {
		t.Errorf("expected JitterFactor=0.3, got %f", result.Resilience.Retry.JitterFactor)
	}
}

func TestResolveProviders_MergesProviders(t *testing.T) {
	cfg := buildDefaultConfig()
	cfg.Resilience.Retry.MaxRetries = 5
	maxRetries := 10
	rawProviders := map[string]rawProviderConfig{
		"openai": {
			Type:   "openai",
			APIKey: "sk-test",
			Resilience: &rawResilienceConfig{
				Retry: &rawRetryConfig{
					MaxRetries: &maxRetries,
				},
			},
		},
		"anthropic": {
			Type:   "anthropic",
			APIKey: "sk-ant",
		},
	}

	resolveProviders(cfg, rawProviders)

	if cfg.Providers["openai"].Resilience.Retry.MaxRetries != 10 {
		t.Errorf("expected openai MaxRetries=10 (overridden), got %d", cfg.Providers["openai"].Resilience.Retry.MaxRetries)
	}
	if cfg.Providers["anthropic"].Resilience.Retry.MaxRetries != 5 {
		t.Errorf("expected anthropic MaxRetries=5 (global), got %d", cfg.Providers["anthropic"].Resilience.Retry.MaxRetries)
	}
	if cfg.Resilience.Retry.MaxRetries != 5 {
		t.Errorf("expected global Resilience.Retry.MaxRetries=5, got %d", cfg.Resilience.Retry.MaxRetries)
	}
}

func TestLoad_ZeroConfig(t *testing.T) {
	clearAllConfigEnvVars(t)

	withTempDir(t, func(_ string) {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if cfg.Server.Port != "8080" {
			t.Errorf("expected default port 8080, got %s", cfg.Server.Port)
		}
		if len(cfg.Providers) != 0 {
			t.Errorf("expected no providers, got %d", len(cfg.Providers))
		}
	})
}

func TestLoad_YAMLOverridesDefaults(t *testing.T) {
	clearAllConfigEnvVars(t)

	withTempDir(t, func(dir string) {
		yaml := `
server:
  port: "3000"
cache:
  type: "redis"
  redis:
    url: "redis://myhost:6379"
    key: "custom:key"
    ttl: 3600
logging:
  enabled: true
  log_bodies: false
  buffer_size: 500
`
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0644); err != nil {
			t.Fatalf("Failed to write config.yaml: %v", err)
		}

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if cfg.Server.Port != "3000" {
			t.Errorf("expected port 3000, got %s", cfg.Server.Port)
		}
		if cfg.Cache.Type != "redis" {
			t.Errorf("expected cache type redis, got %s", cfg.Cache.Type)
		}
		if cfg.Cache.Redis.URL != "redis://myhost:6379" {
			t.Errorf("expected redis URL redis://myhost:6379, got %s", cfg.Cache.Redis.URL)
		}
		if cfg.Cache.Redis.Key != "custom:key" {
			t.Errorf("expected redis key custom:key, got %s", cfg.Cache.Redis.Key)
		}
		if cfg.Cache.Redis.TTL != 3600 {
			t.Errorf("expected redis TTL 3600, got %d", cfg.Cache.Redis.TTL)
		}
		if !cfg.Logging.Enabled {
			t.Error("expected Logging.Enabled=true from YAML")
		}
		if cfg.Logging.LogBodies {
			t.Error("expected Logging.LogBodies=false from YAML")
		}
		if cfg.Logging.BufferSize != 500 {
			t.Errorf("expected Logging.BufferSize=500, got %d", cfg.Logging.BufferSize)
		}
		// Defaults preserved for unset YAML fields
		if cfg.Logging.FlushInterval != 5 {
			t.Errorf("expected Logging.FlushInterval=5 (default), got %d", cfg.Logging.FlushInterval)
		}
		if cfg.Storage.Type != "sqlite" {
			t.Errorf("expected Storage.Type=sqlite (default), got %s", cfg.Storage.Type)
		}
	})
}

func TestLoad_EnvOverridesYAML(t *testing.T) {
	clearAllConfigEnvVars(t)

	withTempDir(t, func(dir string) {
		yaml := `
server:
  port: "3000"
cache:
  type: "redis"
logging:
  enabled: true
`
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0644); err != nil {
			t.Fatalf("Failed to write config.yaml: %v", err)
		}

		t.Setenv("PORT", "9090")
		t.Setenv("CACHE_TYPE", "local")
		t.Setenv("LOGGING_ENABLED", "false")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if cfg.Server.Port != "9090" {
			t.Errorf("expected port 9090 (env override), got %s", cfg.Server.Port)
		}
		if cfg.Cache.Type != "local" {
			t.Errorf("expected cache type local (env override), got %s", cfg.Cache.Type)
		}
		if cfg.Logging.Enabled {
			t.Error("expected Logging.Enabled=false (env override)")
		}
	})
}

func TestLoad_EnvOverridesDefaults(t *testing.T) {
	clearAllConfigEnvVars(t)

	withTempDir(t, func(_ string) {
		t.Setenv("PORT", "5555")
		t.Setenv("STORAGE_TYPE", "postgresql")
		t.Setenv("POSTGRES_URL", "postgres://localhost/test")
		t.Setenv("POSTGRES_MAX_CONNS", "20")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if cfg.Server.Port != "5555" {
			t.Errorf("expected port 5555, got %s", cfg.Server.Port)
		}
		if cfg.Storage.Type != "postgresql" {
			t.Errorf("expected storage type postgresql, got %s", cfg.Storage.Type)
		}
		if cfg.Storage.PostgreSQL.URL != "postgres://localhost/test" {
			t.Errorf("expected postgres URL, got %s", cfg.Storage.PostgreSQL.URL)
		}
		if cfg.Storage.PostgreSQL.MaxConns != 20 {
			t.Errorf("expected max conns 20, got %d", cfg.Storage.PostgreSQL.MaxConns)
		}
	})
}

func TestLoad_ProviderFromEnv(t *testing.T) {
	clearAllConfigEnvVars(t)

	withTempDir(t, func(_ string) {
		t.Setenv("OPENAI_API_KEY", "sk-test-key")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		provider, exists := cfg.Providers["openai"]
		if !exists {
			t.Fatal("expected 'openai' provider to exist")
		}
		if provider.Type != "openai" {
			t.Errorf("expected provider type 'openai', got %s", provider.Type)
		}
		if provider.APIKey != "sk-test-key" {
			t.Errorf("expected API key sk-test-key, got %s", provider.APIKey)
		}
		expected := DefaultRetryConfig()
		if provider.Resilience.Retry != expected {
			t.Errorf("expected provider to inherit global retry defaults\ngot:  %+v\nwant: %+v", provider.Resilience.Retry, expected)
		}
	})
}

func TestLoad_ProviderFromYAML(t *testing.T) {
	clearAllConfigEnvVars(t)

	withTempDir(t, func(dir string) {
		yaml := `
providers:
  openai:
    type: openai
    api_key: "sk-yaml-key"
    base_url: "https://custom.openai.com"
`
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0644); err != nil {
			t.Fatalf("Failed to write config.yaml: %v", err)
		}

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		provider, exists := cfg.Providers["openai"]
		if !exists {
			t.Fatal("expected 'openai' provider to exist")
		}
		if provider.APIKey != "sk-yaml-key" {
			t.Errorf("expected API key sk-yaml-key, got %s", provider.APIKey)
		}
		if provider.BaseURL != "https://custom.openai.com" {
			t.Errorf("expected base URL https://custom.openai.com, got %s", provider.BaseURL)
		}
	})
}

func TestLoad_ProviderResilienceOverrideFromYAML(t *testing.T) {
	clearAllConfigEnvVars(t)

	withTempDir(t, func(dir string) {
		yamlContent := `
resilience:
  retry:
    max_retries: 5
providers:
  openai:
    type: openai
    api_key: "sk-yaml-key"
    resilience:
      retry:
        max_retries: 10
  anthropic:
    type: anthropic
    api_key: "sk-ant-key"
`
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to write config.yaml: %v", err)
		}

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if cfg.Resilience.Retry.MaxRetries != 5 {
			t.Errorf("expected global MaxRetries=5, got %d", cfg.Resilience.Retry.MaxRetries)
		}
		openai := cfg.Providers["openai"]
		if openai.Resilience.Retry.MaxRetries != 10 {
			t.Errorf("expected openai MaxRetries=10 (override), got %d", openai.Resilience.Retry.MaxRetries)
		}
		anthropic := cfg.Providers["anthropic"]
		if anthropic.Resilience.Retry.MaxRetries != 5 {
			t.Errorf("expected anthropic MaxRetries=5 (global), got %d", anthropic.Resilience.Retry.MaxRetries)
		}
	})
}

func TestLoad_EnvOverridesProviderYAML(t *testing.T) {
	clearAllConfigEnvVars(t)

	withTempDir(t, func(dir string) {
		yaml := `
providers:
  openai:
    type: openai
    api_key: "sk-yaml-key"
`
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0644); err != nil {
			t.Fatalf("Failed to write config.yaml: %v", err)
		}

		t.Setenv("OPENAI_API_KEY", "sk-env-key")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		provider, exists := cfg.Providers["openai"]
		if !exists {
			t.Fatal("expected 'openai' provider to exist")
		}
		if provider.APIKey != "sk-env-key" {
			t.Errorf("expected API key sk-env-key (env override), got %s", provider.APIKey)
		}
	})
}

func TestLoad_OllamaNoAPIKey(t *testing.T) {
	clearAllConfigEnvVars(t)

	withTempDir(t, func(_ string) {
		t.Setenv("OLLAMA_BASE_URL", "http://localhost:11434/v1")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		provider, exists := cfg.Providers["ollama"]
		if !exists {
			t.Fatal("expected 'ollama' provider to exist")
		}
		if provider.Type != "ollama" {
			t.Errorf("expected provider type 'ollama', got %s", provider.Type)
		}
		if provider.BaseURL != "http://localhost:11434/v1" {
			t.Errorf("expected base URL, got %s", provider.BaseURL)
		}
	})
}

func TestLoad_EmptyProviderFiltered(t *testing.T) {
	clearAllConfigEnvVars(t)

	withTempDir(t, func(_ string) {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if len(cfg.Providers) != 0 {
			t.Errorf("expected no providers, got %d: %v", len(cfg.Providers), cfg.Providers)
		}
	})
}

func TestLoad_HTTPConfig(t *testing.T) {
	clearAllConfigEnvVars(t)

	// Test defaults
	withTempDir(t, func(_ string) {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if cfg.HTTP.Timeout != 600 {
			t.Errorf("expected HTTP.Timeout=600, got %d", cfg.HTTP.Timeout)
		}
		if cfg.HTTP.ResponseHeaderTimeout != 600 {
			t.Errorf("expected HTTP.ResponseHeaderTimeout=600, got %d", cfg.HTTP.ResponseHeaderTimeout)
		}
	})

	// Test env override
	withTempDir(t, func(_ string) {
		t.Setenv("HTTP_TIMEOUT", "30")
		t.Setenv("HTTP_RESPONSE_HEADER_TIMEOUT", "60")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if cfg.HTTP.Timeout != 30 {
			t.Errorf("expected HTTP.Timeout=30, got %d", cfg.HTTP.Timeout)
		}
		if cfg.HTTP.ResponseHeaderTimeout != 60 {
			t.Errorf("expected HTTP.ResponseHeaderTimeout=60, got %d", cfg.HTTP.ResponseHeaderTimeout)
		}
	})
}

func TestLoad_CacheDir(t *testing.T) {
	clearAllConfigEnvVars(t)

	// Test default
	withTempDir(t, func(_ string) {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if cfg.Cache.CacheDir != ".cache" {
			t.Errorf("expected Cache.CacheDir=.cache, got %s", cfg.Cache.CacheDir)
		}
	})

	// Test env override
	withTempDir(t, func(_ string) {
		t.Setenv("GOMODEL_CACHE_DIR", "/tmp/gomodel-cache")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if cfg.Cache.CacheDir != "/tmp/gomodel-cache" {
			t.Errorf("expected Cache.CacheDir=/tmp/gomodel-cache, got %s", cfg.Cache.CacheDir)
		}
	})
}

func TestLoad_MultipleProviders(t *testing.T) {
	clearAllConfigEnvVars(t)

	withTempDir(t, func(_ string) {
		t.Setenv("OPENAI_API_KEY", "sk-openai")
		t.Setenv("ANTHROPIC_API_KEY", "sk-anthropic")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if _, ok := cfg.Providers["openai"]; !ok {
			t.Error("expected 'openai' provider")
		}
		if _, ok := cfg.Providers["anthropic"]; !ok {
			t.Error("expected 'anthropic' provider")
		}
		if cfg.Providers["openai"].APIKey != "sk-openai" {
			t.Errorf("expected OpenAI key sk-openai, got %s", cfg.Providers["openai"].APIKey)
		}
		if cfg.Providers["anthropic"].APIKey != "sk-anthropic" {
			t.Errorf("expected Anthropic key sk-anthropic, got %s", cfg.Providers["anthropic"].APIKey)
		}
	})
}

func TestLoad_LoggingOnlyModelInteractionsDefault(t *testing.T) {
	clearAllConfigEnvVars(t)

	withTempDir(t, func(_ string) {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if !cfg.Logging.OnlyModelInteractions {
			t.Error("expected OnlyModelInteractions to default to true")
		}
	})
}

func TestLoad_LoggingOnlyModelInteractionsFromEnv(t *testing.T) {
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
			clearAllConfigEnvVars(t)

			withTempDir(t, func(_ string) {
				t.Setenv("LOGGING_ONLY_MODEL_INTERACTIONS", tt.envValue)

				cfg, err := Load()
				if err != nil {
					t.Fatalf("Load() failed: %v", err)
				}

				if cfg.Logging.OnlyModelInteractions != tt.expected {
					t.Errorf("expected OnlyModelInteractions=%v for env value %q, got %v",
						tt.expected, tt.envValue, cfg.Logging.OnlyModelInteractions)
				}
			})
		})
	}
}

func TestLoad_YAMLWithEnvVarExpansion(t *testing.T) {
	clearAllConfigEnvVars(t)

	withTempDir(t, func(dir string) {
		yaml := `
server:
  port: "${TEST_PORT_CFG:-9999}"
providers:
  openai:
    type: "openai"
    api_key: "${TEST_KEY_CFG:-default-key}"
`
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0644); err != nil {
			t.Fatalf("Failed to write config.yaml: %v", err)
		}

		// Test with defaults (env vars not set)
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if cfg.Server.Port != "9999" {
			t.Errorf("expected port 9999 (YAML default), got %s", cfg.Server.Port)
		}
		provider := cfg.Providers["openai"]
		if provider.APIKey != "default-key" {
			t.Errorf("expected API key 'default-key', got %s", provider.APIKey)
		}
	})
}

func TestLoad_YAMLWithEnvVarOverride(t *testing.T) {
	clearAllConfigEnvVars(t)

	withTempDir(t, func(dir string) {
		yaml := `
server:
  port: "${TEST_PORT_CFG:-9999}"
providers:
  openai:
    type: "openai"
    api_key: "${TEST_KEY_CFG:-default-key}"
`
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0644); err != nil {
			t.Fatalf("Failed to write config.yaml: %v", err)
		}

		t.Setenv("TEST_PORT_CFG", "1111")
		t.Setenv("TEST_KEY_CFG", "real-key")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if cfg.Server.Port != "1111" {
			t.Errorf("expected port 1111 (env override), got %s", cfg.Server.Port)
		}
		provider := cfg.Providers["openai"]
		if provider.APIKey != "real-key" {
			t.Errorf("expected API key 'real-key', got %s", provider.APIKey)
		}
	})
}

func TestLoad_YAMLInConfigSubdir(t *testing.T) {
	clearAllConfigEnvVars(t)

	withTempDir(t, func(dir string) {
		configDir := filepath.Join(dir, "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		yaml := `
server:
  port: "4444"
`
		if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yaml), 0644); err != nil {
			t.Fatalf("Failed to write config/config.yaml: %v", err)
		}

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if cfg.Server.Port != "4444" {
			t.Errorf("expected port 4444 from config/config.yaml, got %s", cfg.Server.Port)
		}
	})
}

func TestLoad_UnexpandedProviderFiltered(t *testing.T) {
	clearAllConfigEnvVars(t)

	withTempDir(t, func(dir string) {
		yaml := `
providers:
  openai:
    type: openai
    api_key: "${OPENAI_API_KEY}"
`
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0644); err != nil {
			t.Fatalf("Failed to write config.yaml: %v", err)
		}

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		if _, exists := cfg.Providers["openai"]; exists {
			t.Error("expected openai provider with unexpanded ${OPENAI_API_KEY} to be filtered out")
		}
	})
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
