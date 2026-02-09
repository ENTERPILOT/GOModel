// Package config provides configuration management for the application.
package config

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"

	"gomodel/internal/storage"
)

// Body size limit constants
const (
	DefaultBodySizeLimit int64 = 10 * 1024 * 1024  // 10MB
	MinBodySizeLimit     int64 = 1 * 1024          // 1KB
	MaxBodySizeLimit     int64 = 100 * 1024 * 1024 // 100MB
)

// bodySizeLimitRegex validates body size limit format: digits followed by optional K/M/G unit and optional B suffix
var bodySizeLimitRegex = regexp.MustCompile(`(?i)^(\d+)([KMG])?B?$`)

// Config holds the application configuration
type Config struct {
	Server    ServerConfig            `yaml:"server"`
	Cache     CacheConfig             `yaml:"cache"`
	Storage   StorageConfig           `yaml:"storage"`
	Logging   LogConfig               `yaml:"logging"`
	Usage     UsageConfig             `yaml:"usage"`
	Metrics   MetricsConfig           `yaml:"metrics"`
	HTTP      HTTPConfig              `yaml:"http"`
	Providers map[string]ProviderConfig `yaml:"providers"`
}

// HTTPConfig holds HTTP client configuration for upstream API requests.
// These values are also readable via the HTTP_TIMEOUT and HTTP_RESPONSE_HEADER_TIMEOUT
// environment variables in internal/httpclient/client.go.
type HTTPConfig struct {
	// Timeout is the overall HTTP request timeout in seconds (default: 600)
	Timeout int `yaml:"timeout" env:"HTTP_TIMEOUT"`

	// ResponseHeaderTimeout is the time to wait for response headers in seconds (default: 600)
	ResponseHeaderTimeout int `yaml:"response_header_timeout" env:"HTTP_RESPONSE_HEADER_TIMEOUT"`
}

// LogConfig holds audit logging configuration
type LogConfig struct {
	// Enabled controls whether audit logging is active
	// Default: false
	Enabled bool `yaml:"enabled" env:"LOGGING_ENABLED"`

	// LogBodies enables logging of full request/response bodies
	// WARNING: May contain sensitive data (PII, API keys in prompts)
	// Default: true
	LogBodies bool `yaml:"log_bodies" env:"LOGGING_LOG_BODIES"`

	// LogHeaders enables logging of request/response headers
	// Sensitive headers (Authorization, Cookie, etc.) are auto-redacted
	// Default: true
	LogHeaders bool `yaml:"log_headers" env:"LOGGING_LOG_HEADERS"`

	// BufferSize is the number of log entries to buffer before flushing
	// Default: 1000
	BufferSize int `yaml:"buffer_size" env:"LOGGING_BUFFER_SIZE"`

	// FlushInterval is how often to flush buffered logs (in seconds)
	// Default: 5
	FlushInterval int `yaml:"flush_interval" env:"LOGGING_FLUSH_INTERVAL"`

	// RetentionDays is how long to keep logs (0 = forever)
	// Default: 30
	RetentionDays int `yaml:"retention_days" env:"LOGGING_RETENTION_DAYS"`

	// OnlyModelInteractions limits audit logging to AI model endpoints only
	// When true, only /v1/chat/completions and /v1/responses are logged
	// Endpoints like /health, /metrics, /admin, /v1/models are skipped
	// Default: true
	OnlyModelInteractions bool `yaml:"only_model_interactions" env:"LOGGING_ONLY_MODEL_INTERACTIONS"`
}

// UsageConfig holds token usage tracking configuration
type UsageConfig struct {
	// Enabled controls whether usage tracking is active
	// Default: true
	Enabled bool `yaml:"enabled" env:"USAGE_ENABLED"`

	// EnforceReturningUsageData controls whether to enforce returning usage data in streaming responses.
	// When true, stream_options: {"include_usage": true} is automatically added to streaming requests.
	// Default: true
	EnforceReturningUsageData bool `yaml:"enforce_returning_usage_data" env:"ENFORCE_RETURNING_USAGE_DATA"`

	// BufferSize is the number of usage entries to buffer before flushing
	// Default: 1000
	BufferSize int `yaml:"buffer_size" env:"USAGE_BUFFER_SIZE"`

	// FlushInterval is how often to flush buffered usage entries (in seconds)
	// Default: 5
	FlushInterval int `yaml:"flush_interval" env:"USAGE_FLUSH_INTERVAL"`

	// RetentionDays is how long to keep usage data (0 = forever)
	// Default: 90
	RetentionDays int `yaml:"retention_days" env:"USAGE_RETENTION_DAYS"`
}

// StorageConfig holds database storage configuration (used by audit logging, usage tracking, future IAM, etc.)
type StorageConfig struct {
	// Type specifies the storage backend: "sqlite" (default), "postgresql", or "mongodb"
	Type string `yaml:"type" env:"STORAGE_TYPE"`

	// SQLite configuration
	SQLite SQLiteStorageConfig `yaml:"sqlite"`

	// PostgreSQL configuration
	PostgreSQL PostgreSQLStorageConfig `yaml:"postgresql"`

	// MongoDB configuration
	MongoDB MongoDBStorageConfig `yaml:"mongodb"`
}

// SQLiteStorageConfig holds SQLite-specific storage configuration
type SQLiteStorageConfig struct {
	// Path is the database file path (default: data/gomodel.db)
	Path string `yaml:"path" env:"SQLITE_PATH"`
}

// PostgreSQLStorageConfig holds PostgreSQL-specific storage configuration
type PostgreSQLStorageConfig struct {
	// URL is the connection string (e.g., postgres://user:pass@localhost/dbname)
	URL string `yaml:"url" env:"POSTGRES_URL"`
	// MaxConns is the maximum connection pool size (default: 10)
	MaxConns int `yaml:"max_conns" env:"POSTGRES_MAX_CONNS"`
}

// MongoDBStorageConfig holds MongoDB-specific storage configuration
type MongoDBStorageConfig struct {
	// URL is the connection string (e.g., mongodb://localhost:27017)
	URL string `yaml:"url" env:"MONGODB_URL"`
	// Database is the database name (default: gomodel)
	Database string `yaml:"database" env:"MONGODB_DATABASE"`
}

// CacheConfig holds cache configuration for model storage
type CacheConfig struct {
	// Type specifies the cache backend: "local" (default) or "redis"
	Type string `yaml:"type" env:"CACHE_TYPE"`

	// CacheDir is the directory for local cache files (default: ".cache")
	CacheDir string `yaml:"cache_dir" env:"GOMODEL_CACHE_DIR"`

	// Redis configuration (only used when Type is "redis")
	Redis RedisConfig `yaml:"redis"`
}

// RedisConfig holds Redis-specific configuration
type RedisConfig struct {
	// URL is the Redis connection URL (e.g., "redis://localhost:6379")
	URL string `yaml:"url" env:"REDIS_URL"`

	// Key is the Redis key for storing the model cache (default: "gomodel:models")
	Key string `yaml:"key" env:"REDIS_KEY"`

	// TTL is the time-to-live for cached data in seconds (default: 86400 = 24 hours)
	TTL int `yaml:"ttl" env:"REDIS_TTL"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port          string `yaml:"port" env:"PORT"`
	MasterKey     string `yaml:"master_key" env:"GOMODEL_MASTER_KEY"`         // Optional: Master key for authentication
	BodySizeLimit string `yaml:"body_size_limit" env:"BODY_SIZE_LIMIT"` // Max request body size (e.g., "10M", "1024K")
}

// MetricsConfig holds observability configuration for Prometheus metrics
type MetricsConfig struct {
	// Enabled controls whether Prometheus metrics are collected and exposed
	// Default: false
	Enabled bool `yaml:"enabled" env:"METRICS_ENABLED"`

	// Endpoint is the HTTP path where metrics are exposed
	// Default: "/metrics"
	Endpoint string `yaml:"endpoint" env:"METRICS_ENDPOINT"`
}

// ProviderConfig holds generic provider configuration
type ProviderConfig struct {
	Type    string   `yaml:"type"`     // e.g., "openai", "anthropic", "gemini"
	APIKey  string   `yaml:"api_key"`  // API key for authentication
	BaseURL string   `yaml:"base_url"` // Optional: override default base URL
	Models  []string `yaml:"models"`   // Optional: restrict to specific models
}

// defaultConfig returns the single source of truth for all configuration defaults.
func defaultConfig() Config {
	return Config{
		Server: ServerConfig{Port: "8080"},
		Cache: CacheConfig{
			Type:     "local",
			CacheDir: ".cache",
			Redis: RedisConfig{
				Key: "gomodel:models",
				TTL: 86400,
			},
		},
		Storage: StorageConfig{
			Type: "sqlite",
			SQLite: SQLiteStorageConfig{
				Path: storage.DefaultSQLitePath,
			},
			PostgreSQL: PostgreSQLStorageConfig{
				MaxConns: 10,
			},
			MongoDB: MongoDBStorageConfig{
				Database: "gomodel",
			},
		},
		Logging: LogConfig{
			LogBodies:             true,
			LogHeaders:            true,
			BufferSize:            1000,
			FlushInterval:         5,
			RetentionDays:         30,
			OnlyModelInteractions: true,
		},
		Usage: UsageConfig{
			Enabled:                   true,
			EnforceReturningUsageData: true,
			BufferSize:                1000,
			FlushInterval:             5,
			RetentionDays:             90,
		},
		Metrics: MetricsConfig{
			Endpoint: "/metrics",
		},
		HTTP: HTTPConfig{
			Timeout:               600,
			ResponseHeaderTimeout: 600,
		},
		Providers: make(map[string]ProviderConfig),
	}
}

// Load reads configuration from file and environment using a three-layer pipeline:
//
//	defaults (code) → config.yaml (optional overlay) → env vars (always win)
//
// Every run follows the same code path regardless of whether config.yaml exists.
func Load() (*Config, error) {
	// 1. Load .env into process env (ignore if not found)
	_ = godotenv.Load()

	// 2. Start with compiled defaults
	cfg := defaultConfig()

	// 3. Optional YAML overlay
	if err := applyYAML(&cfg); err != nil {
		return nil, err
	}

	// 4. Env vars always win
	if err := applyEnvOverrides(&cfg); err != nil {
		return nil, err
	}

	// 5. Discover providers from env
	applyProviderEnvVars(&cfg)

	// 6. Filter invalid providers
	removeEmptyProviders(&cfg)

	// 7. Validate
	if cfg.Server.BodySizeLimit != "" {
		if err := ValidateBodySizeLimit(cfg.Server.BodySizeLimit); err != nil {
			return nil, fmt.Errorf("invalid BODY_SIZE_LIMIT: %w", err)
		}
	}

	return &cfg, nil
}

// applyYAML reads an optional config.yaml and overlays it onto cfg.
// If no config file is found, this is a no-op (not an error).
func applyYAML(cfg *Config) error {
	// Search paths: config/config.yaml then ./config.yaml
	paths := []string{
		"config/config.yaml",
		"config.yaml",
	}

	var data []byte
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err == nil {
			data = raw
			break
		}
	}

	if data == nil {
		return nil // No config file found — not an error
	}

	// Expand ${VAR} and ${VAR:-default} before YAML parsing
	expanded := expandString(string(data))

	// Unmarshal into the existing cfg — unset YAML fields preserve defaults
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return fmt.Errorf("failed to parse config.yaml: %w", err)
	}

	// Ensure Providers map is initialized even if YAML had none
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderConfig)
	}

	return nil
}

// applyEnvOverrides walks cfg's struct fields and applies env var overrides
// based on `env` struct tags. Maps are skipped (providers are handled separately).
func applyEnvOverrides(cfg *Config) error {
	return applyEnvOverridesValue(reflect.ValueOf(cfg).Elem())
}

func applyEnvOverridesValue(v reflect.Value) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldVal := v.Field(i)

		// Skip maps — providers are handled by applyProviderEnvVars
		if field.Type.Kind() == reflect.Map {
			continue
		}
		// Recurse into nested structs
		if field.Type.Kind() == reflect.Struct {
			if err := applyEnvOverridesValue(fieldVal); err != nil {
				return err
			}
			continue
		}

		envKey := field.Tag.Get("env")
		if envKey == "" {
			continue
		}
		envVal := os.Getenv(envKey)
		if envVal == "" {
			continue
		}

		switch field.Type.Kind() {
		case reflect.String:
			fieldVal.SetString(envVal)
		case reflect.Bool:
			fieldVal.SetBool(parseBool(envVal))
		case reflect.Int:
			n, err := strconv.Atoi(envVal)
			if err != nil {
				return fmt.Errorf("invalid value for %s (%s): %q is not a valid integer", field.Name, envKey, envVal)
			}
			fieldVal.SetInt(int64(n))
		}
	}
	return nil
}

// knownProvider describes a provider that can be auto-discovered from environment variables.
type knownProvider struct {
	apiKeyEnv    string
	baseURLEnv   string
	name         string
	providerType string
}

var knownProviders = []knownProvider{
	{"OPENAI_API_KEY", "OPENAI_BASE_URL", "openai", "openai"},
	{"ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "anthropic", "anthropic"},
	{"GEMINI_API_KEY", "GEMINI_BASE_URL", "gemini", "gemini"},
	{"XAI_API_KEY", "XAI_BASE_URL", "xai", "xai"},
	{"GROQ_API_KEY", "GROQ_BASE_URL", "groq", "groq"},
	{"OLLAMA_API_KEY", "OLLAMA_BASE_URL", "ollama", "ollama"},
}

// applyProviderEnvVars discovers providers from well-known environment variables.
// Env vars override YAML-provided values for the same provider name.
func applyProviderEnvVars(cfg *Config) {
	for _, kp := range knownProviders {
		apiKey := os.Getenv(kp.apiKeyEnv)
		baseURL := os.Getenv(kp.baseURLEnv)

		// Skip if no env vars set for this provider
		if apiKey == "" && baseURL == "" {
			continue
		}

		// Ollama special case: no API key required, enabled via base URL
		if kp.providerType == "ollama" && apiKey == "" && baseURL == "" {
			continue
		}

		existing, exists := cfg.Providers[kp.name]
		if exists {
			// Override existing provider's env-sourced values
			if apiKey != "" {
				existing.APIKey = apiKey
			}
			if baseURL != "" {
				existing.BaseURL = baseURL
			}
			cfg.Providers[kp.name] = existing
		} else {
			// Add new provider from env
			cfg.Providers[kp.name] = ProviderConfig{
				Type:    kp.providerType,
				APIKey:  apiKey,
				BaseURL: baseURL,
			}
		}
	}
}

// removeEmptyProviders removes providers that have no valid credentials.
func removeEmptyProviders(cfg *Config) {
	for name, pCfg := range cfg.Providers {
		// Preserve Ollama providers with a non-empty BaseURL (no API key required)
		if pCfg.Type == "ollama" && pCfg.BaseURL != "" {
			continue
		}
		// Remove provider if API key is empty or contains unexpanded placeholders
		if pCfg.APIKey == "" || strings.Contains(pCfg.APIKey, "${") {
			delete(cfg.Providers, name)
		}
	}
}

// expandString expands environment variable references like ${VAR} or ${VAR:-default} in a string.
func expandString(s string) string {
	if s == "" {
		return s
	}
	return os.Expand(s, func(key string) string {
		varname := key
		defaultValue := ""
		hasDefault := false
		if idx := strings.Index(key, ":-"); idx >= 0 {
			varname = key[:idx]
			defaultValue = key[idx+2:]
			hasDefault = true
		}
		value := os.Getenv(varname)
		if value == "" {
			if hasDefault {
				return defaultValue
			}
			return "${" + key + "}"
		}
		return value
	})
}

// parseBool returns true if s is "true" or "1" (case-insensitive).
func parseBool(s string) bool {
	return strings.EqualFold(s, "true") || s == "1"
}

// ValidateBodySizeLimit validates a body size limit string.
// Accepts formats like: "10M", "10MB", "1024K", "1024KB", "104857600"
// Returns an error if the format is invalid or value is outside bounds (1KB - 100MB).
func ValidateBodySizeLimit(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	matches := bodySizeLimitRegex.FindStringSubmatch(s)
	if matches == nil {
		return fmt.Errorf("invalid format %q: expected pattern like '10M', '1024K', or '104857600'", s)
	}

	value, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid number in %q: %w", s, err)
	}

	// Apply unit multiplier (case-insensitive due to regex flag)
	switch strings.ToUpper(matches[2]) {
	case "K":
		value *= 1024
	case "M":
		value *= 1024 * 1024
	case "G":
		value *= 1024 * 1024 * 1024
	}

	// Validate bounds
	if value < MinBodySizeLimit {
		return fmt.Errorf("value %d bytes is below minimum of %d bytes (1KB)", value, MinBodySizeLimit)
	}
	if value > MaxBodySizeLimit {
		return fmt.Errorf("value %d bytes exceeds maximum of %d bytes (100MB)", value, MaxBodySizeLimit)
	}

	return nil
}
