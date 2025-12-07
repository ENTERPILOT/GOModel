// Package config provides configuration management for the application.
package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	Server    ServerConfig              `mapstructure:"server"`
	Providers map[string]ProviderConfig `mapstructure:"providers"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port string `mapstructure:"port"`
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
	// Load .env file using Viper (optional, won't fail if not found)
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	_ = viper.ReadInConfig() // Ignore error if .env file doesn't exist

	// Set defaults
	viper.SetDefault("server.port", "8080")

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
				Port: viper.GetString("PORT"),
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

	// Expand provider configurations
	for name, pCfg := range cfg.Providers {
		pCfg.APIKey = expandString(pCfg.APIKey)
		pCfg.BaseURL = expandString(pCfg.BaseURL)
		cfg.Providers[name] = pCfg
	}

	return cfg
}

// expandString expands environment variable references like ${VAR_NAME} in a string
func expandString(s string) string {
	if s == "" {
		return s
	}
	return os.Expand(s, func(key string) string {
		// Try to get from environment
		value := os.Getenv(key)
		if value == "" {
			// If not in environment, return the original placeholder
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
		if pCfg.APIKey != "" && !strings.HasPrefix(pCfg.APIKey, "${") {
			filteredProviders[name] = pCfg
		}
	}
	cfg.Providers = filteredProviders
	return cfg
}
