package providers

import (
	"context"
	"io"
	"testing"
	"time"

	"gomodel/config"
	"gomodel/internal/core"
	"gomodel/internal/llmclient"
)

var _ ProviderConstructor = func(_ string, _ ProviderOptions) core.Provider { return nil }

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

func (m *factoryMockProvider) Embeddings(_ context.Context, _ *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	return &core.EmbeddingResponse{}, nil
}

func TestProviderFactory_Register(t *testing.T) {
	factory := NewProviderFactory()

	factory.Add(Registration{
		Type: "test-provider",
		New: func(apiKey string, opts ProviderOptions) core.Provider {
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

func TestProviderFactory_Add_PanicsOnEmptyType(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for empty Type, got none")
		}
	}()
	NewProviderFactory().Add(Registration{
		Type: "",
		New:  func(_ string, _ ProviderOptions) core.Provider { return nil },
	})
}

func TestProviderFactory_Add_PanicsOnNilConstructor(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil New, got none")
		}
	}()
	NewProviderFactory().Add(Registration{Type: "test", New: nil})
}

func TestProviderFactory_Create_UnknownType(t *testing.T) {
	factory := NewProviderFactory()

	cfg := ProviderConfig{
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

	factory.Add(Registration{
		Type: "mock",
		New: func(apiKey string, opts ProviderOptions) core.Provider {
			return &factoryMockProvider{}
		},
	})

	cfg := ProviderConfig{
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

	for _, name := range []string{"provider1", "provider2", "provider3"} {
		factory.Add(Registration{
			Type: name,
			New: func(apiKey string, opts ProviderOptions) core.Provider {
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

	type mockWithBaseURL struct {
		factoryMockProvider
	}
	mockProvider := &mockWithBaseURL{}

	factory.Add(Registration{
		Type: "custom",
		New: func(apiKey string, opts ProviderOptions) core.Provider {
			return mockProvider
		},
	})

	cfg := ProviderConfig{
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

	mockHooks := llmclient.Hooks{
		OnRequestStart: func(ctx context.Context, info llmclient.RequestInfo) context.Context {
			return ctx
		},
	}
	factory.SetHooks(mockHooks)

	var receivedOpts ProviderOptions
	factory.Add(Registration{
		Type: "test",
		New: func(apiKey string, opts ProviderOptions) core.Provider {
			receivedOpts = opts
			return &factoryMockProvider{}
		},
	})

	cfg := ProviderConfig{
		Type:   "test",
		APIKey: "test-key",
	}

	_, err := factory.Create(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedOpts.Hooks.OnRequestStart == nil {
		t.Error("expected hooks to be passed to builder via ProviderOptions")
	}
}

func TestProviderFactory_HooksPassedToBuilder(t *testing.T) {
	factory := NewProviderFactory()

	mockHooks := llmclient.Hooks{
		OnRequestStart: func(ctx context.Context, info llmclient.RequestInfo) context.Context {
			return ctx
		},
	}
	factory.SetHooks(mockHooks)

	var receivedOpts ProviderOptions

	factory.Add(Registration{
		Type: "test",
		New: func(apiKey string, opts ProviderOptions) core.Provider {
			receivedOpts = opts
			return &factoryMockProvider{}
		},
	})

	cfg := ProviderConfig{
		Type:   "test",
		APIKey: "test-key",
	}

	_, err := factory.Create(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedOpts.Hooks.OnRequestStart == nil {
		t.Error("expected hooks to be passed to builder via ProviderOptions")
	}
}

func TestProviderFactory_ZeroHooks(t *testing.T) {
	factory := NewProviderFactory()

	var receivedOpts ProviderOptions

	factory.Add(Registration{
		Type: "test",
		New: func(apiKey string, opts ProviderOptions) core.Provider {
			receivedOpts = opts
			return &factoryMockProvider{}
		},
	})

	cfg := ProviderConfig{
		Type:   "test",
		APIKey: "test-key",
	}

	_, err := factory.Create(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedOpts.Hooks.OnRequestStart != nil || receivedOpts.Hooks.OnRequestEnd != nil {
		t.Error("expected zero hooks when SetHooks not called")
	}
}

func TestProviderFactory_Create_PassesResilienceConfig(t *testing.T) {
	factory := NewProviderFactory()

	var receivedOpts ProviderOptions
	factory.Add(Registration{
		Type: "test",
		New: func(apiKey string, opts ProviderOptions) core.Provider {
			receivedOpts = opts
			return &factoryMockProvider{}
		},
	})

	resilience := config.ResilienceConfig{
		Retry: config.RetryConfig{
			MaxRetries:     7,
			InitialBackoff: 2 * time.Second,
			MaxBackoff:     60 * time.Second,
			BackoffFactor:  3.0,
			JitterFactor:   0.5,
		},
	}

	cfg := ProviderConfig{
		Type:       "test",
		APIKey:     "test-key",
		Resilience: resilience,
	}

	_, err := factory.Create(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := receivedOpts.Resilience.Retry
	if r.MaxRetries != 7 {
		t.Errorf("MaxRetries = %d, want 7", r.MaxRetries)
	}
	if r.InitialBackoff != 2*time.Second {
		t.Errorf("InitialBackoff = %v, want 2s", r.InitialBackoff)
	}
	if r.MaxBackoff != 60*time.Second {
		t.Errorf("MaxBackoff = %v, want 60s", r.MaxBackoff)
	}
	if r.BackoffFactor != 3.0 {
		t.Errorf("BackoffFactor = %f, want 3.0", r.BackoffFactor)
	}
	if r.JitterFactor != 0.5 {
		t.Errorf("JitterFactor = %f, want 0.5", r.JitterFactor)
	}
}

func TestProviderFactory_Create_SetsProviderName(t *testing.T) {
	factory := NewProviderFactory()
	factory.Add(Registration{
		Type: "testprovider",
		New: func(_ string, _ ProviderOptions) core.Provider {
			return &factoryMockProvider{}
		},
	})

	provider, err := factory.Create(ProviderConfig{Type: "testprovider", APIKey: "key"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()

	chatResp, err := provider.ChatCompletion(ctx, &core.ChatRequest{Model: "m"})
	if err != nil {
		t.Fatalf("ChatCompletion error: %v", err)
	}
	if chatResp.Provider != "testprovider" {
		t.Errorf("ChatCompletion Provider = %q, want %q", chatResp.Provider, "testprovider")
	}

	respResp, err := provider.Responses(ctx, &core.ResponsesRequest{Model: "m", Input: "hi"})
	if err != nil {
		t.Fatalf("Responses error: %v", err)
	}
	if respResp.Provider != "testprovider" {
		t.Errorf("Responses Provider = %q, want %q", respResp.Provider, "testprovider")
	}

	embedResp, err := provider.Embeddings(ctx, &core.EmbeddingRequest{Model: "m", Input: "hi"})
	if err != nil {
		t.Fatalf("Embeddings error: %v", err)
	}
	if embedResp.Provider != "testprovider" {
		t.Errorf("Embeddings Provider = %q, want %q", embedResp.Provider, "testprovider")
	}
}
