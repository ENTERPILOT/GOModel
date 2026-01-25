// Package config provides configuration management for the application.
package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
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
	Server    ServerConfig
	Cache     CacheConfig
	Storage   StorageConfig
	Logging   LogConfig
	Usage     UsageConfig
	Metrics   MetricsConfig
	Providers map[string]ProviderConfig
}

// LogConfig holds audit logging configuration
type LogConfig struct {
	// Enabled controls whether audit logging is active
	// Default: false
	Enabled bool

	// LogBodies enables logging of full request/response bodies
	// WARNING: May contain sensitive data (PII, API keys in prompts)
	// Default: true
	LogBodies bool

	// LogHeaders enables logging of request/response headers
	// Sensitive headers (Authorization, Cookie, etc.) are auto-redacted
	// Default: true
	LogHeaders bool

	// BufferSize is the number of log entries to buffer before flushing
	// Default: 1000
	BufferSize int

	// FlushInterval is how often to flush buffered logs (in seconds)
	// Default: 5
	FlushInterval int

	// RetentionDays is how long to keep logs (0 = forever)
	// Default: 30
	RetentionDays int

	// OnlyModelInteractions limits audit logging to AI model endpoints only
	// When true, only /v1/chat/completions and /v1/responses are logged
	// Endpoints like /health, /metrics, /admin, /v1/models are skipped
	// Default: true
	OnlyModelInteractions bool
}

// UsageConfig holds token usage tracking configuration
type UsageConfig struct {
	// Enabled controls whether usage tracking is active
	// Default: true
	Enabled bool

	// EnforceReturningUsageData controls whether to enforce returning usage data in streaming responses.
	// When true, stream_options: {"include_usage": true} is automatically added to streaming requests.
	// Default: true
	EnforceReturningUsageData bool

	// BufferSize is the number of usage entries to buffer before flushing
	// Default: 1000
	BufferSize int

	// FlushInterval is how often to flush buffered usage entries (in seconds)
	// Default: 5
	FlushInterval int

	// RetentionDays is how long to keep usage data (0 = forever)
	// Default: 90
	RetentionDays int
}

// StorageConfig holds database storage configuration (used by audit logging, usage tracking, future IAM, etc.)
type StorageConfig struct {
	// Type specifies the storage backend: "sqlite" (default), "postgresql", or "mongodb"
	Type string

	// SQLite configuration
	SQLite SQLiteStorageConfig

	// PostgreSQL configuration
	PostgreSQL PostgreSQLStorageConfig

	// MongoDB configuration
	MongoDB MongoDBStorageConfig
}

// SQLiteStorageConfig holds SQLite-specific storage configuration
type SQLiteStorageConfig struct {
	// Path is the database file path (default: .cache/gomodel.db)
	Path string
}

// PostgreSQLStorageConfig holds PostgreSQL-specific storage configuration
type PostgreSQLStorageConfig struct {
	// URL is the connection string (e.g., postgres://user:pass@localhost/dbname)
	URL string
	// MaxConns is the maximum connection pool size (default: 10)
	MaxConns int
}

// MongoDBStorageConfig holds MongoDB-specific storage configuration
type MongoDBStorageConfig struct {
	// URL is the connection string (e.g., mongodb://localhost:27017)
	URL string
	// Database is the database name (default: gomodel)
	Database string
}

// CacheConfig holds cache configuration for model storage
type CacheConfig struct {
	// Type specifies the cache backend: "local" (default) or "redis"
	Type string

	// Redis configuration (only used when Type is "redis")
	Redis RedisConfig
}

// RedisConfig holds Redis-specific configuration
type RedisConfig struct {
	// URL is the Redis connection URL (e.g., "redis://localhost:6379")
	URL string

	// Key is the Redis key for storing the model cache (default: "gomodel:models")
	Key string

	// TTL is the time-to-live for cached data in seconds (default: 86400 = 24 hours)
	TTL int
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port          string
	MasterKey     string // Optional: Master key for authentication
	BodySizeLimit string // Max request body size (e.g., "10M", "1024K")
}

// MetricsConfig holds observability configuration for Prometheus metrics
type MetricsConfig struct {
	// Enabled controls whether Prometheus metrics are collected and exposed
	// Default: false
	Enabled bool

	// Endpoint is the HTTP path where metrics are exposed
	// Default: "/metrics"
	Endpoint string
}

// ProviderConfig holds generic provider configuration
type ProviderConfig struct {
	Type    string   // e.g., "openai", "anthropic", "gemini"
	APIKey  string   // API key for authentication
	BaseURL string   // Optional: override default base URL
	Models  []string // Optional: restrict to specific models
}

// snakeCaseMatchName returns a DecoderConfigOption that matches
// snake_case map keys to PascalCase struct field names.
func snakeCaseMatchName() viper.DecoderConfigOption {
	return func(c *mapstructure.DecoderConfig) {
		c.MatchName = func(mapKey, fieldName string) bool {
			// Reject malformed keys: leading/trailing underscores or consecutive underscores
			if strings.HasPrefix(mapKey, "_") || strings.HasSuffix(mapKey, "_") {
				return false
			}
			if strings.Contains(mapKey, "__") {
				return false
			}

			// Remove underscores and compare case-insensitively
			normalizedKey := strings.ReplaceAll(mapKey, "_", "")
			return strings.EqualFold(normalizedKey, fieldName)
		}
	}
}

// Load reads configuration from file and environment
func Load() (*Config, error) {
	// Load .env file directly into environment variables
	// This ensures os.Getenv works for variables defined in .env
	_ = godotenv.Load() // Ignore error (e.g., file not found)

	// Load .env file using Viper (optional, won't fail if not found)
	viper.SetConfigName(".env")

	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	_ = viper.ReadInConfig() // Ignore error if .env file doesn't exist

	// Set defaults
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("cache.type", "local")
	viper.SetDefault("cache.redis.key", "gomodel:models")
	viper.SetDefault("cache.redis.ttl", 86400) // 24 hours
	viper.SetDefault("metrics.enabled", false)
	viper.SetDefault("metrics.endpoint", "/metrics")

	// Storage defaults
	viper.SetDefault("storage.type", "sqlite")
	viper.SetDefault("storage.sqlite.path", ".cache/gomodel.db")
	viper.SetDefault("storage.postgresql.max_conns", 10)
	viper.SetDefault("storage.mongodb.database", "gomodel")

	// Logging defaults
	viper.SetDefault("logging.enabled", false)
	viper.SetDefault("logging.log_bodies", true)
	viper.SetDefault("logging.log_headers", true)
	viper.SetDefault("logging.buffer_size", 1000)
	viper.SetDefault("logging.flush_interval", 5)
	viper.SetDefault("logging.retention_days", 30)
	viper.SetDefault("logging.only_model_interactions", true)

	// Usage tracking defaults
	viper.SetDefault("usage.enabled", true)
	viper.SetDefault("usage.enforce_returning_usage_data", true)
	viper.SetDefault("usage.buffer_size", 1000)
	viper.SetDefault("usage.flush_interval", 5)
	viper.SetDefault("usage.retention_days", 90)

	// Enable automatic environment variable reading
	viper.AutomaticEnv()

	// Try to read config.yaml
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	var cfg Config

	// Read config file (optional, won't fail if not found)
	if err := viper.ReadInConfig(); err == nil {
		// Config file found, unmarshal it
		if err := viper.Unmarshal(&cfg, snakeCaseMatchName()); err != nil {
			return nil, err
		}
		// Expand environment variables in config values
		cfg = expandEnvVars(cfg)
		// Remove providers with unresolved environment variables
		cfg = removeEmptyProviders(cfg)
	} else {
		// No config file, use environment variables (legacy support)
		cfg = Config{
			Server: ServerConfig{
				Port:          viper.GetString("PORT"),
				MasterKey:     viper.GetString("GOMODEL_MASTER_KEY"),
				BodySizeLimit: viper.GetString("BODY_SIZE_LIMIT"),
			},
			Storage: StorageConfig{
			Type: getEnvOrDefault("STORAGE_TYPE", "sqlite"),
				SQLite: SQLiteStorageConfig{
					Path: getEnvOrDefault("SQLITE_PATH", ".cache/gomodel.db"),
				},
				PostgreSQL: PostgreSQLStorageConfig{
					URL:      os.Getenv("POSTGRES_URL"),
					MaxConns: getEnvIntOrDefault("POSTGRES_MAX_CONNS", 10),
				},
				MongoDB: MongoDBStorageConfig{
					URL:      os.Getenv("MONGODB_URL"),
					Database: getEnvOrDefault("MONGODB_DATABASE", "gomodel"),
				},
			},
			Logging: LogConfig{
				Enabled:               getEnvBool("LOGGING_ENABLED"),
				LogBodies:             getEnvBoolOrDefault("LOGGING_LOG_BODIES", true),
				LogHeaders:            getEnvBoolOrDefault("LOGGING_LOG_HEADERS", true),
				BufferSize:            getEnvIntOrDefault("LOGGING_BUFFER_SIZE", 1000),
				FlushInterval:         getEnvIntOrDefault("LOGGING_FLUSH_INTERVAL", 5),
				RetentionDays:         getEnvIntOrDefault("LOGGING_RETENTION_DAYS", 30),
				OnlyModelInteractions: getEnvBoolOrDefault("LOGGING_ONLY_MODEL_INTERACTIONS", true),
			},
			Usage: UsageConfig{
				Enabled:                   getEnvBoolOrDefault("USAGE_ENABLED", true),
				EnforceReturningUsageData: getEnvBoolOrDefault("ENFORCE_RETURNING_USAGE_DATA", true),
				BufferSize:                getEnvIntOrDefault("USAGE_BUFFER_SIZE", 1000),
				FlushInterval:             getEnvIntOrDefault("USAGE_FLUSH_INTERVAL", 5),
				RetentionDays:             getEnvIntOrDefault("USAGE_RETENTION_DAYS", 90),
			},
			Metrics: MetricsConfig{
				Enabled:  viper.GetBool("METRICS_ENABLED"),
				Endpoint: viper.GetString("METRICS_ENDPOINT"),
			},
			Providers: make(map[string]ProviderConfig),
		}

		// TODO: Similarly for ENV variables. All ENV variables like *_API_KEY should be taken and iterated over
		// Add providers from environment variables if available
		if apiKey := viper.GetString("OPENAI_API_KEY"); apiKey != "" {
			cfg.Providers["openai"] = ProviderConfig{
				Type:   "openai",
				APIKey: apiKey,
			}
		}
		if apiKey := viper.GetString("ANTHROPIC_API_KEY"); apiKey != "" {
			cfg.Providers["anthropic"] = ProviderConfig{
				Type:   "anthropic",
				APIKey: apiKey,
			}
		}
		if apiKey := viper.GetString("GEMINI_API_KEY"); apiKey != "" {
			cfg.Providers["gemini"] = ProviderConfig{
				Type:   "gemini",
				APIKey: apiKey,
			}
		}
		if apiKey := viper.GetString("XAI_API_KEY"); apiKey != "" {
			cfg.Providers["xai"] = ProviderConfig{
				Type:   "xai",
				APIKey: apiKey,
			}
		}
		if apiKey := viper.GetString("GROQ_API_KEY"); apiKey != "" {
			cfg.Providers["groq"] = ProviderConfig{
				Type:   "groq",
				APIKey: apiKey,
			}
		}
		// Ollama (no API key required, enabled via base URL)
		if baseURL := viper.GetString("OLLAMA_BASE_URL"); baseURL != "" {
			cfg.Providers["ollama"] = ProviderConfig{
				Type:    "ollama",
				APIKey:  "", // Not required
				BaseURL: baseURL,
			}
		}
	}

	// Validate body size limit if provided
	if cfg.Server.BodySizeLimit != "" {
		if err := ValidateBodySizeLimit(cfg.Server.BodySizeLimit); err != nil {
			return nil, fmt.Errorf("invalid BODY_SIZE_LIMIT: %w", err)
		}
	}

	return &cfg, nil
}

// expandEnvVars expands environment variable references in configuration values
func expandEnvVars(cfg Config) Config {
	// Expand server config
	cfg.Server.Port = expandString(cfg.Server.Port)
	cfg.Server.MasterKey = expandString(cfg.Server.MasterKey)
	cfg.Server.BodySizeLimit = expandString(cfg.Server.BodySizeLimit)

	// Expand metrics configuration
	// Check METRICS_ENABLED env var - it should override YAML config
	if metricsEnabled := os.Getenv("METRICS_ENABLED"); metricsEnabled != "" {
		cfg.Metrics.Enabled = strings.EqualFold(metricsEnabled, "true") || metricsEnabled == "1"
	}
	cfg.Metrics.Endpoint = expandString(cfg.Metrics.Endpoint)

	// Expand cache configuration
	cfg.Cache.Type = expandString(cfg.Cache.Type)
	cfg.Cache.Redis.URL = expandString(cfg.Cache.Redis.URL)
	cfg.Cache.Redis.Key = expandString(cfg.Cache.Redis.Key)

	// Expand storage configuration
	cfg.Storage.Type = expandString(cfg.Storage.Type)
	cfg.Storage.SQLite.Path = expandString(cfg.Storage.SQLite.Path)
	cfg.Storage.PostgreSQL.URL = expandString(cfg.Storage.PostgreSQL.URL)
	cfg.Storage.MongoDB.URL = expandString(cfg.Storage.MongoDB.URL)
	cfg.Storage.MongoDB.Database = expandString(cfg.Storage.MongoDB.Database)

	// Override storage configuration from environment variables
	// This allows env vars to take precedence over config file values
	if storageType := os.Getenv("STORAGE_TYPE"); storageType != "" {
		cfg.Storage.Type = storageType
	}
	if sqlitePath := os.Getenv("SQLITE_PATH"); sqlitePath != "" {
		cfg.Storage.SQLite.Path = sqlitePath
	}
	if postgresURL := os.Getenv("POSTGRES_URL"); postgresURL != "" {
		cfg.Storage.PostgreSQL.URL = postgresURL
	}
	if postgresMaxConns := os.Getenv("POSTGRES_MAX_CONNS"); postgresMaxConns != "" {
		if maxConns, err := strconv.Atoi(postgresMaxConns); err == nil {
			cfg.Storage.PostgreSQL.MaxConns = maxConns
		}
	}
	if mongoURL := os.Getenv("MONGODB_URL"); mongoURL != "" {
		cfg.Storage.MongoDB.URL = mongoURL
	}
	if mongoDatabase := os.Getenv("MONGODB_DATABASE"); mongoDatabase != "" {
		cfg.Storage.MongoDB.Database = mongoDatabase
	}

	// Override logging configuration from environment variables
	if loggingEnabled := os.Getenv("LOGGING_ENABLED"); loggingEnabled != "" {
		cfg.Logging.Enabled = strings.EqualFold(loggingEnabled, "true") || loggingEnabled == "1"
	}
	if logBodies := os.Getenv("LOGGING_LOG_BODIES"); logBodies != "" {
		cfg.Logging.LogBodies = strings.EqualFold(logBodies, "true") || logBodies == "1"
	}
	if logHeaders := os.Getenv("LOGGING_LOG_HEADERS"); logHeaders != "" {
		cfg.Logging.LogHeaders = strings.EqualFold(logHeaders, "true") || logHeaders == "1"
	}
	if onlyModel := os.Getenv("LOGGING_ONLY_MODEL_INTERACTIONS"); onlyModel != "" {
		cfg.Logging.OnlyModelInteractions = strings.EqualFold(onlyModel, "true") || onlyModel == "1"
	}

	// Override usage tracking configuration from environment variables
	if usageEnabled := os.Getenv("USAGE_ENABLED"); usageEnabled != "" {
		cfg.Usage.Enabled = strings.EqualFold(usageEnabled, "true") || usageEnabled == "1"
	}
	if enforceUsage := os.Getenv("ENFORCE_RETURNING_USAGE_DATA"); enforceUsage != "" {
		cfg.Usage.EnforceReturningUsageData = strings.EqualFold(enforceUsage, "true") || enforceUsage == "1"
	}
	if usageBufferSize := os.Getenv("USAGE_BUFFER_SIZE"); usageBufferSize != "" {
		if bufferSize, err := strconv.Atoi(usageBufferSize); err == nil {
			cfg.Usage.BufferSize = bufferSize
		}
	}
	if usageFlushInterval := os.Getenv("USAGE_FLUSH_INTERVAL"); usageFlushInterval != "" {
		if flushInterval, err := strconv.Atoi(usageFlushInterval); err == nil {
			cfg.Usage.FlushInterval = flushInterval
		}
	}
	if usageRetentionDays := os.Getenv("USAGE_RETENTION_DAYS"); usageRetentionDays != "" {
		if retentionDays, err := strconv.Atoi(usageRetentionDays); err == nil {
			cfg.Usage.RetentionDays = retentionDays
		}
	}

	// Expand provider configurations
	for name, pCfg := range cfg.Providers {
		pCfg.APIKey = expandString(pCfg.APIKey)
		pCfg.BaseURL = expandString(pCfg.BaseURL)
		cfg.Providers[name] = pCfg
	}

	return cfg
}

// expandString expands environment variable references like ${VAR_NAME} or ${VAR_NAME:-default} in a string
func expandString(s string) string {
	if s == "" {
		return s
	}
	return os.Expand(s, func(key string) string {
		// Check for default value syntax ${VAR:-default}
		varname := key
		defaultValue := ""
		hasDefault := false
		if strings.Contains(key, ":-") {
			parts := strings.SplitN(key, ":-", 2)
			varname = parts[0]
			defaultValue = parts[1]
			hasDefault = true
		}

		// Try to get from environment
		value := os.Getenv(varname)
		if value == "" {
			// If default syntax was used (even with empty default), return the default
			if hasDefault {
				return defaultValue
			}
			// If not in environment and no default syntax, return the original placeholder
			// This allows config to work with or without env vars
			return "${" + key + "}"
		}
		return value
	})
}

// removeEmptyProviders removes providers with empty API keys
func removeEmptyProviders(cfg Config) Config {
	filteredProviders := make(map[string]ProviderConfig)
	for name, pCfg := range cfg.Providers {
		// Preserve Ollama providers with a non-empty BaseURL (no API key required)
		if pCfg.Type == "ollama" && pCfg.BaseURL != "" {
			filteredProviders[name] = pCfg
			continue
		}
		// Keep provider only if API key doesn't contain unexpanded placeholders
		if pCfg.APIKey != "" && !strings.Contains(pCfg.APIKey, "${") {
			filteredProviders[name] = pCfg
		}
	}
	cfg.Providers = filteredProviders
	return cfg
}

// getEnvOrDefault returns the environment variable value or the default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntOrDefault returns the environment variable as int or the default if not set/invalid
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvBool returns true if the environment variable is "true" or "1"
func getEnvBool(key string) bool {
	value := os.Getenv(key)
	return strings.EqualFold(value, "true") || value == "1"
}

// getEnvBoolOrDefault returns the boolean value of the environment variable,
// or the default value if the variable is not set
func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return strings.EqualFold(value, "true") || value == "1"
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
