package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestLoad_DefaultPort(t *testing.T) {
	// Reset viper state before test
	viper.Reset()
	
	// Clear any existing environment variables
	os.Unsetenv("PORT")
	os.Unsetenv("OPENAI_API_KEY")

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
	os.Setenv("PORT", "9090")
	defer os.Unsetenv("PORT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.Port != "9090" {
		t.Errorf("expected port 9090 from env, got %s", cfg.Server.Port)
	}
}

func TestLoad_OpenAIAPIKeyFromEnv(t *testing.T) {
	// Reset viper state before test
	viper.Reset()
	
	// Set environment variable
	testAPIKey := "sk-test-key-12345"
	os.Setenv("OPENAI_API_KEY", testAPIKey)
	defer os.Unsetenv("OPENAI_API_KEY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.OpenAI.APIKey != testAPIKey {
		t.Errorf("expected API key %s from env, got %s", testAPIKey, cfg.OpenAI.APIKey)
	}
}

func TestLoad_EmptyAPIKey(t *testing.T) {
	// Reset viper state before test
	viper.Reset()
	
	// Clear environment variable
	os.Unsetenv("OPENAI_API_KEY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.OpenAI.APIKey != "" {
		t.Errorf("expected empty API key, got %s", cfg.OpenAI.APIKey)
	}
}

func TestLoad_MultipleEnvVars(t *testing.T) {
	// Reset viper state before test
	viper.Reset()
	
	// Set multiple environment variables
	testPort := "3000"
	testAPIKey := "sk-test-multiple"
	
	os.Setenv("PORT", testPort)
	os.Setenv("OPENAI_API_KEY", testAPIKey)
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("OPENAI_API_KEY")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.Port != testPort {
		t.Errorf("expected port %s, got %s", testPort, cfg.Server.Port)
	}

	if cfg.OpenAI.APIKey != testAPIKey {
		t.Errorf("expected API key %s, got %s", testAPIKey, cfg.OpenAI.APIKey)
	}
}

func TestLoad_DotEnvFile(t *testing.T) {
	// Reset viper state before test
	viper.Reset()
	
	// Clear environment variables to test .env file reading
	os.Unsetenv("PORT")
	os.Unsetenv("OPENAI_API_KEY")

	// Create a temporary .env file
	envContent := `PORT=7070
OPENAI_API_KEY=sk-from-dotenv-file
`
	err := os.WriteFile(".env.test", []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}
	defer os.Remove(".env.test")

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
		OpenAI: OpenAIConfig{
			APIKey: viper.GetString("OPENAI_API_KEY"),
		},
	}

	// Verify values from .env file
	if cfg.Server.Port != "7070" {
		t.Errorf("expected port 7070 from .env file, got %s", cfg.Server.Port)
	}

	if cfg.OpenAI.APIKey != "sk-from-dotenv-file" {
		t.Errorf("expected API key from .env file, got %s", cfg.OpenAI.APIKey)
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
	defer os.Remove(".env.test2")

	// Set environment variables (should override .env file)
	os.Setenv("PORT", "9999")
	os.Setenv("OPENAI_API_KEY", "sk-from-real-env")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("OPENAI_API_KEY")
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
		OpenAI: OpenAIConfig{
			APIKey: viper.GetString("OPENAI_API_KEY"),
		},
	}

	// Environment variables should override .env file
	if cfg.Server.Port != "9999" {
		t.Errorf("expected port 9999 from environment variable (not .env file), got %s", cfg.Server.Port)
	}

	if cfg.OpenAI.APIKey != "sk-from-real-env" {
		t.Errorf("expected API key from environment variable (not .env file), got %s", cfg.OpenAI.APIKey)
	}
}

