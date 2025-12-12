package providers

import (
	"context"
	"errors"
	"io"
	"testing"

	"gomodel/internal/core"
)

// mockProvider is a simple mock implementation of core.Provider for testing
type mockProvider struct {
	name              string
	chatResponse      *core.ChatResponse
	responsesResponse *core.ResponsesResponse
	modelsResponse    *core.ModelsResponse
	err               error
}

func (m *mockProvider) ChatCompletion(_ context.Context, _ *core.ChatRequest) (*core.ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.chatResponse, nil
}

func (m *mockProvider) StreamChatCompletion(_ context.Context, _ *core.ChatRequest) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	return io.NopCloser(nil), nil
}

func (m *mockProvider) ListModels(_ context.Context) (*core.ModelsResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.modelsResponse, nil
}

func (m *mockProvider) Responses(_ context.Context, _ *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.responsesResponse, nil
}

func (m *mockProvider) StreamResponses(_ context.Context, _ *core.ResponsesRequest) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	return io.NopCloser(nil), nil
}

// createTestRegistry creates a registry with mock providers for testing
func createTestRegistry(providers ...*mockProvider) *ModelRegistry {
	registry := NewModelRegistry()
	for _, p := range providers {
		registry.RegisterProvider(p)
	}
	// Initialize the registry to populate models from providers
	_ = registry.Initialize(context.Background())
	return registry
}

func TestRouterSupports(t *testing.T) {
	openaiMock := &mockProvider{
		name: "openai",
		modelsResponse: &core.ModelsResponse{
			Object: "list",
			Data: []core.Model{
				{ID: "gpt-4o", Object: "model", OwnedBy: "openai"},
			},
		},
	}
	anthropicMock := &mockProvider{
		name: "anthropic",
		modelsResponse: &core.ModelsResponse{
			Object: "list",
			Data: []core.Model{
				{ID: "claude-3-5-sonnet-20241022", Object: "model", OwnedBy: "anthropic"},
			},
		},
	}

	registry := createTestRegistry(openaiMock, anthropicMock)
	router := NewRouter(registry)

	tests := []struct {
		model    string
		expected bool
	}{
		{"gpt-4o", true},
		{"claude-3-5-sonnet-20241022", true},
		{"unsupported-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := router.Supports(tt.model)
			if result != tt.expected {
				t.Errorf("Supports(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestRouterChatCompletion(t *testing.T) {
	openaiResp := &core.ChatResponse{ID: "openai-response", Model: "gpt-4o"}
	anthropicResp := &core.ChatResponse{ID: "anthropic-response", Model: "claude-3-5-sonnet-20241022"}

	openaiMock := &mockProvider{
		name:         "openai",
		chatResponse: openaiResp,
		modelsResponse: &core.ModelsResponse{
			Object: "list",
			Data: []core.Model{
				{ID: "gpt-4o", Object: "model", OwnedBy: "openai"},
			},
		},
	}
	anthropicMock := &mockProvider{
		name:         "anthropic",
		chatResponse: anthropicResp,
		modelsResponse: &core.ModelsResponse{
			Object: "list",
			Data: []core.Model{
				{ID: "claude-3-5-sonnet-20241022", Object: "model", OwnedBy: "anthropic"},
			},
		},
	}

	registry := createTestRegistry(openaiMock, anthropicMock)
	router := NewRouter(registry)

	tests := []struct {
		name          string
		model         string
		expectedResp  *core.ChatResponse
		expectedError bool
	}{
		{
			name:         "route to openai",
			model:        "gpt-4o",
			expectedResp: openaiResp,
		},
		{
			name:         "route to anthropic",
			model:        "claude-3-5-sonnet-20241022",
			expectedResp: anthropicResp,
		},
		{
			name:          "unsupported model",
			model:         "unsupported-model",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &core.ChatRequest{
				Model: tt.model,
				Messages: []core.Message{
					{Role: "user", Content: "test"},
				},
			}

			resp, err := router.ChatCompletion(context.Background(), req)

			if tt.expectedError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if resp != tt.expectedResp {
					t.Errorf("got response ID %q, want %q", resp.ID, tt.expectedResp.ID)
				}
			}
		})
	}
}

func TestRouterListModels(t *testing.T) {
	openaiMock := &mockProvider{
		name: "openai",
		modelsResponse: &core.ModelsResponse{
			Object: "list",
			Data: []core.Model{
				{ID: "gpt-4o", Object: "model", OwnedBy: "openai"},
			},
		},
	}
	anthropicMock := &mockProvider{
		name: "anthropic",
		modelsResponse: &core.ModelsResponse{
			Object: "list",
			Data: []core.Model{
				{ID: "claude-3-5-sonnet-20241022", Object: "model", OwnedBy: "anthropic"},
			},
		},
	}

	registry := createTestRegistry(openaiMock, anthropicMock)
	router := NewRouter(registry)

	resp, err := router.ListModels(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Errorf("expected 2 models, got %d", len(resp.Data))
	}

	// Verify both providers' models are included
	foundOpenAI := false
	foundAnthropic := false
	for _, model := range resp.Data {
		if model.ID == "gpt-4o" {
			foundOpenAI = true
		}
		if model.ID == "claude-3-5-sonnet-20241022" {
			foundAnthropic = true
		}
	}

	if !foundOpenAI {
		t.Error("OpenAI model not found in combined list")
	}
	if !foundAnthropic {
		t.Error("Anthropic model not found in combined list")
	}
}

func TestRouterListModelsWithError(t *testing.T) {
	// Test that router continues even if one provider fails during initialization
	openaiMock := &mockProvider{
		name: "openai",
		modelsResponse: &core.ModelsResponse{
			Object: "list",
			Data: []core.Model{
				{ID: "gpt-4o", Object: "model", OwnedBy: "openai"},
			},
		},
	}
	anthropicMock := &mockProvider{
		name: "anthropic",
		err:  errors.New("provider error"),
	}

	registry := createTestRegistry(openaiMock, anthropicMock)
	router := NewRouter(registry)

	resp, err := router.ListModels(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still get OpenAI models even though Anthropic failed during initialization
	if len(resp.Data) != 1 {
		t.Errorf("expected 1 model, got %d", len(resp.Data))
	}
	if resp.Data[0].ID != "gpt-4o" {
		t.Errorf("expected gpt-4o, got %s", resp.Data[0].ID)
	}
}

func TestModelRegistry(t *testing.T) {
	t.Run("RegisterProvider", func(t *testing.T) {
		registry := NewModelRegistry()
		mock := &mockProvider{
			name: "test",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "test-model", Object: "model", OwnedBy: "test"},
				},
			},
		}
		registry.RegisterProvider(mock)

		if registry.ProviderCount() != 1 {
			t.Errorf("expected 1 provider, got %d", registry.ProviderCount())
		}
	})

	t.Run("Initialize", func(t *testing.T) {
		registry := NewModelRegistry()
		mock := &mockProvider{
			name: "test",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "test-model-1", Object: "model", OwnedBy: "test"},
					{ID: "test-model-2", Object: "model", OwnedBy: "test"},
				},
			},
		}
		registry.RegisterProvider(mock)

		err := registry.Initialize(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if registry.ModelCount() != 2 {
			t.Errorf("expected 2 models, got %d", registry.ModelCount())
		}
	})

	t.Run("GetProvider", func(t *testing.T) {
		registry := NewModelRegistry()
		mock := &mockProvider{
			name: "test",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "test-model", Object: "model", OwnedBy: "test"},
				},
			},
		}
		registry.RegisterProvider(mock)
		_ = registry.Initialize(context.Background())

		provider := registry.GetProvider("test-model")
		if provider != mock {
			t.Error("expected to get the registered provider")
		}

		provider = registry.GetProvider("unknown-model")
		if provider != nil {
			t.Error("expected nil for unknown model")
		}
	})

	t.Run("Supports", func(t *testing.T) {
		registry := NewModelRegistry()
		mock := &mockProvider{
			name: "test",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "test-model", Object: "model", OwnedBy: "test"},
				},
			},
		}
		registry.RegisterProvider(mock)
		_ = registry.Initialize(context.Background())

		if !registry.Supports("test-model") {
			t.Error("expected Supports to return true for registered model")
		}

		if registry.Supports("unknown-model") {
			t.Error("expected Supports to return false for unknown model")
		}
	})

	t.Run("DuplicateModels", func(t *testing.T) {
		// Test that first provider wins when models have the same ID
		registry := NewModelRegistry()
		mock1 := &mockProvider{
			name: "provider1",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "shared-model", Object: "model", OwnedBy: "provider1"},
				},
			},
		}
		mock2 := &mockProvider{
			name: "provider2",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "shared-model", Object: "model", OwnedBy: "provider2"},
				},
			},
		}
		registry.RegisterProvider(mock1)
		registry.RegisterProvider(mock2)
		_ = registry.Initialize(context.Background())

		// Should only have one model (first provider wins)
		if registry.ModelCount() != 1 {
			t.Errorf("expected 1 model (deduplicated), got %d", registry.ModelCount())
		}

		// First provider should be the one associated with the model
		provider := registry.GetProvider("shared-model")
		if provider != mock1 {
			t.Error("expected first provider to win for duplicate model")
		}
	})
}
