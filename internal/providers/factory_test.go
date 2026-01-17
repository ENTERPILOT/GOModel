package providers

import (
	"context"
	"io"
	"testing"

	"gomodel/config"
	"gomodel/internal/core"
	"gomodel/internal/llmclient"
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

func TestProviderFactory_Register(t *testing.T) {
	factory := NewProviderFactory()

	// Test registering a new provider type
	factory.Register(Registration{
		Type: "test-provider",
		New: func(apiKey string, hooks *llmclient.Hooks) core.Provider {
			return &factoryMockProvider{}
		},
	})

	registered := factory.RegisteredTypes()
	if len(registered) != 1 {
		t.Errorf("expected 1 registered provider, got %d", len(registered))
	}
	if registered[0] != "test-provider" {
		t.Errorf("expected 'test-provider', got %q", registered[0])
	}
}

func TestProviderFactory_Create_UnknownType(t *testing.T) {
	factory := NewProviderFactory()

	cfg := config.ProviderConfig{
		Type:   "unknown-type",
		APIKey: "test-key",
	}

	_, err := factory.Create(cfg)
	if err == nil {
		t.Error("expected error for unknown provider type, got nil")
	}

	expectedMsg := "unknown provider type: unknown-type"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestProviderFactory_Create_Success(t *testing.T) {
	factory := NewProviderFactory()

	// Register a mock builder
	factory.Register(Registration{
		Type: "mock",
		New: func(apiKey string, hooks *llmclient.Hooks) core.Provider {
			return &factoryMockProvider{}
		},
	})

	cfg := config.ProviderConfig{
		Type:   "mock",
		APIKey: "test-key",
	}

	provider, err := factory.Create(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if provider == nil {
		t.Error("expected provider to be created, got nil")
	}
}

func TestProviderFactory_RegisteredTypes(t *testing.T) {
	factory := NewProviderFactory()

	// Register some test providers
	for _, name := range []string{"provider1", "provider2", "provider3"} {
		factory.Register(Registration{
			Type: name,
			New: func(apiKey string, hooks *llmclient.Hooks) core.Provider {
				return &factoryMockProvider{}
			},
		})
	}

	registered := factory.RegisteredTypes()

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

func TestProviderFactory_Create_WithBaseURL(t *testing.T) {
	factory := NewProviderFactory()

	customBaseURL := "https://custom.api.endpoint.com/v1"

	// Create a mock provider
	type mockWithBaseURL struct {
		factoryMockProvider
	}
	mockProvider := &mockWithBaseURL{}

	// Register a mock builder
	factory.Register(Registration{
		Type: "custom",
		New: func(apiKey string, hooks *llmclient.Hooks) core.Provider {
			return mockProvider
		},
	})

	cfg := config.ProviderConfig{
		Type:    "custom",
		APIKey:  "test-key",
		BaseURL: customBaseURL,
	}

	// The factory only calls SetBaseURL if the provider implements it.
	// Our mock doesn't implement it, so we're just testing that Create succeeds.
	provider, err := factory.Create(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if provider == nil {
		t.Error("expected provider to be created, got nil")
	}
}

func TestProviderFactory_SetHooks(t *testing.T) {
	factory := NewProviderFactory()

	// Create mock hooks with identifiable callbacks
	mockHooks := &llmclient.Hooks{
		OnRequestStart: func(ctx context.Context, info llmclient.RequestInfo) context.Context {
			return ctx
		},
	}
	factory.SetHooks(mockHooks)

	// Verify hooks were set by creating a provider and checking it received them
	var receivedHooks *llmclient.Hooks
	factory.Register(Registration{
		Type: "test",
		New: func(apiKey string, hooks *llmclient.Hooks) core.Provider {
			receivedHooks = hooks
			return &factoryMockProvider{}
		},
	})

	cfg := config.ProviderConfig{
		Type:   "test",
		APIKey: "test-key",
	}

	_, err := factory.Create(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify hooks were passed by checking callback exists
	if receivedHooks == nil || receivedHooks.OnRequestStart == nil {
		t.Error("expected hooks to be passed to builder")
	}
}

func TestProviderFactory_HooksPassedToBuilder(t *testing.T) {
	factory := NewProviderFactory()

	// Create mock hooks
	mockHooks := &llmclient.Hooks{
		OnRequestStart: func(ctx context.Context, info llmclient.RequestInfo) context.Context {
			return ctx
		},
	}
	factory.SetHooks(mockHooks)

	var receivedHooks *llmclient.Hooks

	factory.Register(Registration{
		Type: "test",
		New: func(apiKey string, hooks *llmclient.Hooks) core.Provider {
			receivedHooks = hooks
			return &factoryMockProvider{}
		},
	})

	cfg := config.ProviderConfig{
		Type:   "test",
		APIKey: "test-key",
	}

	_, err := factory.Create(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify hooks were passed by checking callback exists
	if receivedHooks == nil || receivedHooks.OnRequestStart == nil {
		t.Error("expected hooks to be passed to builder")
	}
}

func TestProviderFactory_NilHooks(t *testing.T) {
	factory := NewProviderFactory()

	var receivedHooks *llmclient.Hooks

	factory.Register(Registration{
		Type: "test",
		New: func(apiKey string, hooks *llmclient.Hooks) core.Provider {
			receivedHooks = hooks
			return &factoryMockProvider{}
		},
	})

	cfg := config.ProviderConfig{
		Type:   "test",
		APIKey: "test-key",
	}

	_, err := factory.Create(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Without SetHooks being called, hooks should be nil
	if receivedHooks != nil {
		t.Error("expected nil hooks when SetHooks not called")
	}
}
