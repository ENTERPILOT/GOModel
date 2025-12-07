// Package config provides configuration management for the application.
package config

import (
	"os"

	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	Server ServerConfig `mapstructure:"server"`
	OpenAI OpenAIConfig `mapstructure:"openai"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port string `mapstructure:"port"`
}

// OpenAIConfig holds OpenAI-specific configuration
type OpenAIConfig struct {
	APIKey string `mapstructure:"api_key"`
}

// Load reads configuration from file and environment
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	// Set defaults
	viper.SetDefault("server.port", "8088")

	// Read config file (optional, won't fail if not found)
	_ = viper.ReadInConfig() //nolint:errcheck

	// Environment variables override config file
	viper.AutomaticEnv()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Override with environment variable if set
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		cfg.OpenAI.APIKey = apiKey
	}

	return &cfg, nil
}
