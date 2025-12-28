package providers

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"gomodel/internal/core"
)

// mockProvider is a simple mock implementation of core.Provider for testing
type mockProvider struct {
	name              string
	chatResponse      *core.ChatResponse
	responsesResponse *core.ResponsesResponse
	modelsResponse    *core.ModelsResponse
	err               error
	listModelsDelay   time.Duration // optional delay before returning from ListModels
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

func (m *mockProvider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	if m.listModelsDelay > 0 {
		select {
		case <-time.After(m.listModelsDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
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

func TestNewRouterValidation(t *testing.T) {
	t.Run("NilRegistry", func(t *testing.T) {
		router, err := NewRouter(nil)
		if err == nil {
			t.Error("expected error for nil registry")
		}
		if router != nil {
			t.Error("expected nil router for nil registry")
		}
	})

	t.Run("ValidRegistry", func(t *testing.T) {
		registry := NewModelRegistry()
		router, err := NewRouter(registry)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if router == nil {
			t.Error("expected non-nil router")
		}
	})
}

func TestRouterUninitializedRegistry(t *testing.T) {
	// Create a router with an empty registry (no models loaded)
	registry := NewModelRegistry()
	router, err := NewRouter(registry)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}

	t.Run("Supports returns false", func(t *testing.T) {
		if router.Supports("any-model") {
			t.Error("expected Supports to return false for uninitialized registry")
		}
	})

	t.Run("ChatCompletion returns error", func(t *testing.T) {
		req := &core.ChatRequest{Model: "any-model"}
		_, err := router.ChatCompletion(context.Background(), req)
		if err == nil {
			t.Error("expected error for uninitialized registry")
		}
		if !errors.Is(err, ErrRegistryNotInitialized) {
			t.Errorf("expected ErrRegistryNotInitialized, got: %v", err)
		}
	})

	t.Run("StreamChatCompletion returns error", func(t *testing.T) {
		req := &core.ChatRequest{Model: "any-model"}
		_, err := router.StreamChatCompletion(context.Background(), req)
		if err == nil {
			t.Error("expected error for uninitialized registry")
		}
		if !errors.Is(err, ErrRegistryNotInitialized) {
			t.Errorf("expected ErrRegistryNotInitialized, got: %v", err)
		}
	})

	t.Run("ListModels returns error", func(t *testing.T) {
		_, err := router.ListModels(context.Background())
		if err == nil {
			t.Error("expected error for uninitialized registry")
		}
		if !errors.Is(err, ErrRegistryNotInitialized) {
			t.Errorf("expected ErrRegistryNotInitialized, got: %v", err)
		}
	})

	t.Run("Responses returns error", func(t *testing.T) {
		req := &core.ResponsesRequest{Model: "any-model"}
		_, err := router.Responses(context.Background(), req)
		if err == nil {
			t.Error("expected error for uninitialized registry")
		}
		if !errors.Is(err, ErrRegistryNotInitialized) {
			t.Errorf("expected ErrRegistryNotInitialized, got: %v", err)
		}
	})

	t.Run("StreamResponses returns error", func(t *testing.T) {
		req := &core.ResponsesRequest{Model: "any-model"}
		_, err := router.StreamResponses(context.Background(), req)
		if err == nil {
			t.Error("expected error for uninitialized registry")
		}
		if !errors.Is(err, ErrRegistryNotInitialized) {
			t.Errorf("expected ErrRegistryNotInitialized, got: %v", err)
		}
	})
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
	router, err := NewRouter(registry)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}

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
	router, err := NewRouter(registry)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}

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
	router, err := NewRouter(registry)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}

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
	router, err := NewRouter(registry)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}

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

	t.Run("GetModel", func(t *testing.T) {
		registry := NewModelRegistry()
		expectedModel := core.Model{
			ID:      "test-model",
			Object:  "model",
			OwnedBy: "test-provider",
			Created: 1234567890,
		}
		mock := &mockProvider{
			name: "test-provider",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data:   []core.Model{expectedModel},
			},
		}
		registry.RegisterProvider(mock)
		_ = registry.Initialize(context.Background())

		// Test getting a registered model
		modelInfo := registry.GetModel("test-model")
		if modelInfo == nil {
			t.Fatal("expected ModelInfo for registered model, got nil")
		}
		if modelInfo.Model.ID != expectedModel.ID {
			t.Errorf("expected model ID %q, got %q", expectedModel.ID, modelInfo.Model.ID)
		}
		if modelInfo.Model.OwnedBy != expectedModel.OwnedBy {
			t.Errorf("expected model OwnedBy %q, got %q", expectedModel.OwnedBy, modelInfo.Model.OwnedBy)
		}
		if modelInfo.Model.Created != expectedModel.Created {
			t.Errorf("expected model Created %d, got %d", expectedModel.Created, modelInfo.Model.Created)
		}
		if modelInfo.Provider != mock {
			t.Error("expected Provider to be the registered mock provider")
		}

		// Test getting an unknown model
		unknownInfo := registry.GetModel("unknown-model")
		if unknownInfo != nil {
			t.Errorf("expected nil for unknown model, got %+v", unknownInfo)
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

	t.Run("AllProvidersFail", func(t *testing.T) {
		// Test that Initialize returns an error when all providers fail
		registry := NewModelRegistry()
		mock1 := &mockProvider{
			name: "provider1",
			err:  errors.New("provider1 error"),
		}
		mock2 := &mockProvider{
			name: "provider2",
			err:  errors.New("provider2 error"),
		}
		registry.RegisterProvider(mock1)
		registry.RegisterProvider(mock2)

		err := registry.Initialize(context.Background())
		if err == nil {
			t.Error("expected error when all providers fail, got nil")
		}

		expectedMsg := "failed to fetch models from any provider"
		if err.Error() != expectedMsg {
			t.Errorf("expected error message '%s', got '%s'", expectedMsg, err.Error())
		}
	})

	t.Run("ListModelsOrdering", func(t *testing.T) {
		// Test that ListModels returns models in consistent sorted order
		registry := NewModelRegistry()
		mock := &mockProvider{
			name: "test",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "zebra-model", Object: "model", OwnedBy: "test"},
					{ID: "alpha-model", Object: "model", OwnedBy: "test"},
					{ID: "middle-model", Object: "model", OwnedBy: "test"},
				},
			},
		}
		registry.RegisterProvider(mock)
		_ = registry.Initialize(context.Background())

		// Call ListModels multiple times and verify consistent ordering
		for i := 0; i < 5; i++ {
			models := registry.ListModels()
			if len(models) != 3 {
				t.Fatalf("expected 3 models, got %d", len(models))
			}

			// Verify sorted order
			if models[0].ID != "alpha-model" {
				t.Errorf("expected first model to be 'alpha-model', got '%s'", models[0].ID)
			}
			if models[1].ID != "middle-model" {
				t.Errorf("expected second model to be 'middle-model', got '%s'", models[1].ID)
			}
			if models[2].ID != "zebra-model" {
				t.Errorf("expected third model to be 'zebra-model', got '%s'", models[2].ID)
			}
		}
	})

	t.Run("RefreshDoesNotBlockReads", func(t *testing.T) {
		// Test that Refresh builds new map without blocking concurrent reads
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

		// Verify model is available before refresh
		if !registry.Supports("test-model") {
			t.Fatal("expected model to be available before refresh")
		}

		// During refresh, the model should still be accessible
		// (testing the atomic swap behavior)
		err := registry.Refresh(context.Background())
		if err != nil {
			t.Fatalf("unexpected refresh error: %v", err)
		}

		// Model should still be available after refresh
		if !registry.Supports("test-model") {
			t.Error("expected model to be available after refresh")
		}
	})
}

func TestStartBackgroundRefresh(t *testing.T) {
	t.Run("RefreshesAtInterval", func(t *testing.T) {
		var refreshCount atomic.Int32
		mock := &mockProvider{
			name: "test",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "test-model", Object: "model", OwnedBy: "test"},
				},
			},
		}

		// Create a wrapper that counts refreshes
		countingMock := &countingMockProvider{
			mockProvider: mock,
			listCount:    &refreshCount,
		}

		registry := NewModelRegistry()
		registry.RegisterProvider(countingMock)
		_ = registry.Initialize(context.Background())

		// Reset counter after initial initialization
		refreshCount.Store(0)

		// Start background refresh with a short interval
		interval := 50 * time.Millisecond
		cancel := registry.StartBackgroundRefresh(interval)
		defer cancel()

		// Wait for a few refresh cycles
		time.Sleep(interval*3 + 25*time.Millisecond)

		count := refreshCount.Load()
		if count < 2 {
			t.Errorf("expected at least 2 refreshes, got %d", count)
		}
	})

	t.Run("StopsOnCancel", func(t *testing.T) {
		var refreshCount atomic.Int32
		mock := &mockProvider{
			name: "test",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "test-model", Object: "model", OwnedBy: "test"},
				},
			},
		}

		countingMock := &countingMockProvider{
			mockProvider: mock,
			listCount:    &refreshCount,
		}

		registry := NewModelRegistry()
		registry.RegisterProvider(countingMock)
		_ = registry.Initialize(context.Background())

		// Reset counter after initial initialization
		refreshCount.Store(0)

		// Start and immediately cancel
		interval := 50 * time.Millisecond
		cancel := registry.StartBackgroundRefresh(interval)
		cancel()

		// Wait a bit to ensure no more refreshes happen
		time.Sleep(interval * 3)

		count := refreshCount.Load()
		if count > 1 {
			t.Errorf("expected at most 1 refresh after cancel, got %d", count)
		}
	})

	t.Run("HandlesRefreshErrors", func(t *testing.T) {
		var refreshCount atomic.Int32
		mock := &mockProvider{
			name: "failing",
			err:  errors.New("refresh error"),
		}

		countingMock := &countingMockProvider{
			mockProvider: mock,
			listCount:    &refreshCount,
		}

		registry := NewModelRegistry()
		// First add a working provider to initialize successfully
		workingMock := &mockProvider{
			name: "working",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "working-model", Object: "model", OwnedBy: "working"},
				},
			},
		}
		registry.RegisterProvider(workingMock)
		registry.RegisterProvider(countingMock)
		_ = registry.Initialize(context.Background())

		// Reset counter
		refreshCount.Store(0)

		// Start background refresh - should continue even with errors
		interval := 50 * time.Millisecond
		cancel := registry.StartBackgroundRefresh(interval)
		defer cancel()

		// Wait for refresh attempts
		time.Sleep(interval*3 + 25*time.Millisecond)

		// The background refresh should continue attempting even with errors
		count := refreshCount.Load()
		if count < 2 {
			t.Errorf("expected at least 2 refresh attempts despite errors, got %d", count)
		}
	})
}

// countingMockProvider wraps mockProvider and counts ListModels calls
type countingMockProvider struct {
	*mockProvider
	listCount *atomic.Int32
}

func (c *countingMockProvider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	c.listCount.Add(1)
	return c.mockProvider.ListModels(ctx)
}
