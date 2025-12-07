// Package config provides configuration management for the application.
package config

import (
	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	OpenAI    OpenAIConfig    `mapstructure:"openai"`
	Anthropic AnthropicConfig `mapstructure:"anthropic"`
	Gemini    GeminiConfig    `mapstructure:"gemini"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port string `mapstructure:"port"`
}

// OpenAIConfig holds OpenAI-specific configuration
type OpenAIConfig struct {
	APIKey string `mapstructure:"api_key"`
}

// AnthropicConfig holds Anthropic-specific configuration
type AnthropicConfig struct {
	APIKey string `mapstructure:"api_key"`
}

// GeminiConfig holds Google Gemini-specific configuration
type GeminiConfig struct {
	APIKey string `mapstructure:"api_key"`
}

// Load reads configuration from file and environment
func Load() (*Config, error) {
	// Load .env file using Viper (optional, won't fail if not found)
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	_ = viper.ReadInConfig() // Ignore error if .env file doesn't exist

	// Set defaults
	viper.SetDefault("PORT", "8080")

	// Enable automatic environment variable reading
	viper.AutomaticEnv()

	// Commented out: config.yaml reading (not used anymore)
	// viper.SetConfigName("config")
	// viper.SetConfigType("yaml")
	// viper.AddConfigPath("./config")
	// viper.AddConfigPath(".")
	//
	// // Read config file (optional, won't fail if not found)
	// _ = viper.ReadInConfig() //nolint:errcheck
	//
	// var cfg Config
	// if err := viper.Unmarshal(&cfg); err != nil {
	// 	return nil, err
	// }

	// Read configuration from environment variables using Viper
	cfg := &Config{
		Server: ServerConfig{
			Port: viper.GetString("PORT"),
		},
		OpenAI: OpenAIConfig{
			APIKey: viper.GetString("OPENAI_API_KEY"),
		},
		Anthropic: AnthropicConfig{
			APIKey: viper.GetString("ANTHROPIC_API_KEY"),
		},
		Gemini: GeminiConfig{
			APIKey: viper.GetString("GEMINI_API_KEY"),
		},
	}

	return cfg, nil
}
