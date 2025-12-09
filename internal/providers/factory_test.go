package providers

import (
	"context"
	"io"
	"testing"

	"gomodel/config"
	"gomodel/internal/core"
)

// factoryMockProvider is a test implementation of core.Provider
type factoryMockProvider struct {
	supportsFunc func(model string) bool
}

func (m *factoryMockProvider) Supports(model string) bool {
	if m.supportsFunc != nil {
		return m.supportsFunc(model)
	}
	return true
}

func (m *factoryMockProvider) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	return &core.ChatResponse{}, nil
}

func (m *factoryMockProvider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	return nil, nil
}

func (m *factoryMockProvider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	return &core.ModelsResponse{}, nil
}

func (m *factoryMockProvider) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	return &core.ResponsesResponse{}, nil
}

func (m *factoryMockProvider) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	return nil, nil
}

func TestRegister(t *testing.T) {
	// Save current registry and restore after test
	originalRegistry := registry
	defer func() { registry = originalRegistry }()

	// Create a fresh registry for testing
	registry = make(map[string]Builder)

	// Test registering a new provider type
	mockBuilder := func(cfg config.ProviderConfig) (core.Provider, error) {
		return nil, nil
	}

	Register("test-provider", mockBuilder)

	if _, exists := registry["test-provider"]; !exists {
		t.Error("expected 'test-provider' to be registered")
	}

	if len(registry) != 1 {
		t.Errorf("expected registry to have 1 entry, got %d", len(registry))
	}
}

func TestCreate_UnknownType(t *testing.T) {
	// Save current registry and restore after test
	originalRegistry := registry
	defer func() { registry = originalRegistry }()

	// Create a fresh registry for testing
	registry = make(map[string]Builder)

	cfg := config.ProviderConfig{
		Type:   "unknown-type",
		APIKey: "test-key",
	}

	_, err := Create(cfg)
	if err == nil {
		t.Error("expected error for unknown provider type, got nil")
	}

	expectedMsg := "unknown provider type: unknown-type"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestCreate_Success(t *testing.T) {
	// Save current registry and restore after test
	originalRegistry := registry
	defer func() { registry = originalRegistry }()

	// Create a fresh registry for testing
	registry = make(map[string]Builder)

	// Register a mock builder
	Register("mock", func(cfg config.ProviderConfig) (core.Provider, error) {
		return &factoryMockProvider{}, nil
	})

	cfg := config.ProviderConfig{
		Type:   "mock",
		APIKey: "test-key",
	}

	provider, err := Create(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if provider == nil {
		t.Error("expected provider to be created, got nil")
	}
}

func TestListRegistered(t *testing.T) {
	// Save current registry and restore after test
	originalRegistry := registry
	defer func() { registry = originalRegistry }()

	// Create a fresh registry for testing
	registry = make(map[string]Builder)

	// Register some test providers
	Register("provider1", func(cfg config.ProviderConfig) (core.Provider, error) { return nil, nil })
	Register("provider2", func(cfg config.ProviderConfig) (core.Provider, error) { return nil, nil })
	Register("provider3", func(cfg config.ProviderConfig) (core.Provider, error) { return nil, nil })

	registered := ListRegistered()

	if len(registered) != 3 {
		t.Errorf("expected 3 registered providers, got %d", len(registered))
	}

	// Check that all expected types are present
	found := make(map[string]bool)
	for _, name := range registered {
		found[name] = true
	}

	expectedTypes := []string{"provider1", "provider2", "provider3"}
	for _, expected := range expectedTypes {
		if !found[expected] {
			t.Errorf("expected '%s' to be in registered list", expected)
		}
	}
}

func TestCreate_WithBaseURL(t *testing.T) {
	// This test verifies that the factory pattern allows providers
	// to use custom base URLs from configuration
	// (Actual provider implementations are tested in their own packages)

	// Save current registry and restore after test
	originalRegistry := registry
	defer func() { registry = originalRegistry }()

	// Create a fresh registry for testing
	registry = make(map[string]Builder)

	customBaseURL := "https://custom.api.endpoint.com/v1"
	var capturedBaseURL string

	// Register a mock builder that captures the base URL
	Register("custom", func(cfg config.ProviderConfig) (core.Provider, error) {
		capturedBaseURL = cfg.BaseURL
		return nil, nil
	})

	cfg := config.ProviderConfig{
		Type:    "custom",
		APIKey:  "test-key",
		BaseURL: customBaseURL,
	}

	_, err := Create(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if capturedBaseURL != customBaseURL {
		t.Errorf("expected base URL '%s', got '%s'", customBaseURL, capturedBaseURL)
	}
}
