package config

import (
	"os"
	"testing"
)

func TestLoad_FromEnvironment(t *testing.T) {
	_ = os.Setenv("PORT", "9090")
	defer func() {
		_ = os.Unsetenv("PORT")
	}()

	result, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Config.Server.Port != "9090" {
		t.Errorf("expected port 9090, got %s", result.Config.Server.Port)
	}
}
