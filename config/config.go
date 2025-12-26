// Package config provides configuration management for the application.
package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	Server    ServerConfig              `mapstructure:"server"`
	Cache     CacheConfig               `mapstructure:"cache"`
	Metrics   MetricsConfig             `mapstructure:"metrics"`
	Providers map[string]ProviderConfig `mapstructure:"providers"`
}

// CacheConfig holds cache configuration for model storage
type CacheConfig struct {
	// Type specifies the cache backend: "local" (default) or "redis"
	Type string `mapstructure:"type"`

	// Redis configuration (only used when Type is "redis")
	Redis RedisConfig `mapstructure:"redis"`
}

// RedisConfig holds Redis-specific configuration
type RedisConfig struct {
	// URL is the Redis connection URL (e.g., "redis://localhost:6379")
	URL string `mapstructure:"url"`

	// Key is the Redis key for storing the model cache (default: "gomodel:models")
	Key string `mapstructure:"key"`

	// TTL is the time-to-live for cached data in seconds (default: 86400 = 24 hours)
	TTL int `mapstructure:"ttl"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port      string `mapstructure:"port"`
	MasterKey string `mapstructure:"master_key"` // Optional: Master key for authentication
}

// MetricsConfig holds observability configuration for Prometheus metrics
type MetricsConfig struct {
	// Enabled controls whether Prometheus metrics are collected and exposed
	// Default: false
	Enabled bool `mapstructure:"enabled"`

	// Endpoint is the HTTP path where metrics are exposed
	// Default: "/metrics"
	Endpoint string `mapstructure:"endpoint"`
}

// ProviderConfig holds generic provider configuration
type ProviderConfig struct {
	Type    string   `mapstructure:"type"`     // e.g., "openai", "anthropic", "gemini"
	APIKey  string   `mapstructure:"api_key"`  // API key for authentication
	BaseURL string   `mapstructure:"base_url"` // Optional: override default base URL
	Models  []string `mapstructure:"models"`   // Optional: restrict to specific models
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
		if err := viper.Unmarshal(&cfg); err != nil {
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
				Port:      viper.GetString("PORT"),
				MasterKey: viper.GetString("GOMODEL_MASTER_KEY"),
			},
			Metrics: MetricsConfig{
				Enabled:  viper.GetBool("METRICS_ENABLED"),
				Endpoint: viper.GetString("METRICS_ENDPOINT"),
			},
			Providers: make(map[string]ProviderConfig),
		}

		// Add providers from environment variables if available
		if apiKey := viper.GetString("OPENAI_API_KEY"); apiKey != "" {
			cfg.Providers["openai-primary"] = ProviderConfig{
				Type:   "openai",
				APIKey: apiKey,
			}
		}
		if apiKey := viper.GetString("ANTHROPIC_API_KEY"); apiKey != "" {
			cfg.Providers["anthropic-primary"] = ProviderConfig{
				Type:   "anthropic",
				APIKey: apiKey,
			}
		}
		if apiKey := viper.GetString("GEMINI_API_KEY"); apiKey != "" {
			cfg.Providers["gemini-primary"] = ProviderConfig{
				Type:   "gemini",
				APIKey: apiKey,
			}
		}
	}

	return &cfg, nil
}

// expandEnvVars expands environment variable references in configuration values
func expandEnvVars(cfg Config) Config {
	// Expand server port
	cfg.Server.Port = expandString(cfg.Server.Port)
	cfg.Server.MasterKey = expandString(cfg.Server.MasterKey)

	// Expand metrics configuration
	cfg.Metrics.Endpoint = expandString(cfg.Metrics.Endpoint)

	// Expand cache configuration
	cfg.Cache.Type = expandString(cfg.Cache.Type)
	cfg.Cache.Redis.URL = expandString(cfg.Cache.Redis.URL)
	cfg.Cache.Redis.Key = expandString(cfg.Cache.Redis.Key)

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
		// Keep provider only if API key doesn't contain unexpanded placeholders
		if pCfg.APIKey != "" && !strings.Contains(pCfg.APIKey, "${") {
			filteredProviders[name] = pCfg
		}
	}
	cfg.Providers = filteredProviders
	return cfg
}
