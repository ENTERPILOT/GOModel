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

// registryMockProvider is a mock implementation of core.Provider for Registry testing.
// It includes all fields needed for testing the full registry lifecycle.
type registryMockProvider struct {
	name              string
	chatResponse      *core.ChatResponse
	responsesResponse *core.ResponsesResponse
	modelsResponse    *core.ModelsResponse
	err               error
	listModelsDelay   time.Duration
}

func (m *registryMockProvider) ChatCompletion(_ context.Context, _ *core.ChatRequest) (*core.ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.chatResponse, nil
}

func (m *registryMockProvider) StreamChatCompletion(_ context.Context, _ *core.ChatRequest) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	return io.NopCloser(nil), nil
}

func (m *registryMockProvider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
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

func (m *registryMockProvider) Responses(_ context.Context, _ *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.responsesResponse, nil
}

func (m *registryMockProvider) StreamResponses(_ context.Context, _ *core.ResponsesRequest) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	return io.NopCloser(nil), nil
}

func TestModelRegistry(t *testing.T) {
	t.Run("RegisterProvider", func(t *testing.T) {
		registry := NewModelRegistry()
		mock := &registryMockProvider{
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
		mock := &registryMockProvider{
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
		mock := &registryMockProvider{
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
		mock := &registryMockProvider{
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
		mock := &registryMockProvider{
			name: "test-provider",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data:   []core.Model{expectedModel},
			},
		}
		registry.RegisterProvider(mock)
		_ = registry.Initialize(context.Background())

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

		unknownInfo := registry.GetModel("unknown-model")
		if unknownInfo != nil {
			t.Errorf("expected nil for unknown model, got %+v", unknownInfo)
		}
	})

	t.Run("DuplicateModels", func(t *testing.T) {
		registry := NewModelRegistry()
		mock1 := &registryMockProvider{
			name: "provider1",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "shared-model", Object: "model", OwnedBy: "provider1"},
				},
			},
		}
		mock2 := &registryMockProvider{
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

		if registry.ModelCount() != 1 {
			t.Errorf("expected 1 model (deduplicated), got %d", registry.ModelCount())
		}

		provider := registry.GetProvider("shared-model")
		if provider != mock1 {
			t.Error("expected first provider to win for duplicate model")
		}
	})

	t.Run("AllProvidersFail", func(t *testing.T) {
		registry := NewModelRegistry()
		mock1 := &registryMockProvider{
			name: "provider1",
			err:  errors.New("provider1 error"),
		}
		mock2 := &registryMockProvider{
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
		registry := NewModelRegistry()
		mock := &registryMockProvider{
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

		for i := 0; i < 5; i++ {
			models := registry.ListModels()
			if len(models) != 3 {
				t.Fatalf("expected 3 models, got %d", len(models))
			}

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
		registry := NewModelRegistry()
		mock := &registryMockProvider{
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
			t.Fatal("expected model to be available before refresh")
		}

		err := registry.Refresh(context.Background())
		if err != nil {
			t.Fatalf("unexpected refresh error: %v", err)
		}

		if !registry.Supports("test-model") {
			t.Error("expected model to be available after refresh")
		}
	})

	t.Run("GetProviderType", func(t *testing.T) {
		registry := NewModelRegistry()
		mock := &registryMockProvider{
			name: "test",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "test-model", Object: "model", OwnedBy: "test"},
				},
			},
		}
		registry.RegisterProviderWithType(mock, "openai")
		_ = registry.Initialize(context.Background())

		pType := registry.GetProviderType("test-model")
		if pType != "openai" {
			t.Errorf("expected provider type 'openai', got '%s'", pType)
		}

		pType = registry.GetProviderType("unknown-model")
		if pType != "" {
			t.Errorf("expected empty provider type for unknown model, got '%s'", pType)
		}
	})
}

func TestListModelsWithProvider_Empty(t *testing.T) {
	registry := NewModelRegistry()
	models := registry.ListModelsWithProvider()
	if len(models) != 0 {
		t.Errorf("expected empty slice, got %d models", len(models))
	}
}

func TestListModelsWithProvider_Sorted(t *testing.T) {
	registry := NewModelRegistry()

	mock1 := &registryMockProvider{
		name: "provider1",
		modelsResponse: &core.ModelsResponse{
			Object: "list",
			Data: []core.Model{
				{ID: "zebra-model", Object: "model", OwnedBy: "provider1"},
				{ID: "alpha-model", Object: "model", OwnedBy: "provider1"},
			},
		},
	}
	mock2 := &registryMockProvider{
		name: "provider2",
		modelsResponse: &core.ModelsResponse{
			Object: "list",
			Data: []core.Model{
				{ID: "middle-model", Object: "model", OwnedBy: "provider2"},
			},
		},
	}
	registry.RegisterProviderWithType(mock1, "openai")
	registry.RegisterProviderWithType(mock2, "anthropic")
	_ = registry.Initialize(context.Background())

	models := registry.ListModelsWithProvider()
	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(models))
	}
	if models[0].Model.ID != "alpha-model" {
		t.Errorf("expected first model alpha-model, got %s", models[0].Model.ID)
	}
	if models[1].Model.ID != "middle-model" {
		t.Errorf("expected second model middle-model, got %s", models[1].Model.ID)
	}
	if models[2].Model.ID != "zebra-model" {
		t.Errorf("expected third model zebra-model, got %s", models[2].Model.ID)
	}
}

func TestListModelsWithProvider_IncludesProviderType(t *testing.T) {
	registry := NewModelRegistry()

	mock1 := &registryMockProvider{
		name: "provider1",
		modelsResponse: &core.ModelsResponse{
			Object: "list",
			Data: []core.Model{
				{ID: "gpt-4", Object: "model", OwnedBy: "openai"},
			},
		},
	}
	mock2 := &registryMockProvider{
		name: "provider2",
		modelsResponse: &core.ModelsResponse{
			Object: "list",
			Data: []core.Model{
				{ID: "claude-3", Object: "model", OwnedBy: "anthropic"},
			},
		},
	}
	registry.RegisterProviderWithType(mock1, "openai")
	registry.RegisterProviderWithType(mock2, "anthropic")
	_ = registry.Initialize(context.Background())

	models := registry.ListModelsWithProvider()
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}

	// Models are sorted: claude-3 before gpt-4
	if models[0].ProviderType != "anthropic" {
		t.Errorf("expected claude-3 provider type 'anthropic', got %q", models[0].ProviderType)
	}
	if models[1].ProviderType != "openai" {
		t.Errorf("expected gpt-4 provider type 'openai', got %q", models[1].ProviderType)
	}
}

// countingRegistryMockProvider wraps registryMockProvider and counts ListModels calls
type countingRegistryMockProvider struct {
	*registryMockProvider
	listCount *atomic.Int32
}

func (c *countingRegistryMockProvider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	c.listCount.Add(1)
	return c.registryMockProvider.ListModels(ctx)
}

func TestStartBackgroundRefresh(t *testing.T) {
	t.Run("RefreshesAtInterval", func(t *testing.T) {
		var refreshCount atomic.Int32
		mock := &registryMockProvider{
			name: "test",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "test-model", Object: "model", OwnedBy: "test"},
				},
			},
		}

		countingMock := &countingRegistryMockProvider{
			registryMockProvider: mock,
			listCount:            &refreshCount,
		}

		registry := NewModelRegistry()
		registry.RegisterProvider(countingMock)
		_ = registry.Initialize(context.Background())

		refreshCount.Store(0)

		interval := 50 * time.Millisecond
		cancel := registry.StartBackgroundRefresh(interval, "")
		defer cancel()

		time.Sleep(interval*3 + 25*time.Millisecond)

		count := refreshCount.Load()
		if count < 2 {
			t.Errorf("expected at least 2 refreshes, got %d", count)
		}
	})

	t.Run("StopsOnCancel", func(t *testing.T) {
		var refreshCount atomic.Int32
		mock := &registryMockProvider{
			name: "test",
			modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data: []core.Model{
					{ID: "test-model", Object: "model", OwnedBy: "test"},
				},
			},
		}

		countingMock := &countingRegistryMockProvider{
			registryMockProvider: mock,
			listCount:            &refreshCount,
		}

		registry := NewModelRegistry()
		registry.RegisterProvider(countingMock)
		_ = registry.Initialize(context.Background())

		refreshCount.Store(0)

		interval := 50 * time.Millisecond
		cancel := registry.StartBackgroundRefresh(interval, "")
		cancel()

		time.Sleep(interval * 3)

		count := refreshCount.Load()
		if count > 1 {
			t.Errorf("expected at most 1 refresh after cancel, got %d", count)
		}
	})

	t.Run("HandlesRefreshErrors", func(t *testing.T) {
		var refreshCount atomic.Int32
		mock := &registryMockProvider{
			name: "failing",
			err:  errors.New("refresh error"),
		}

		countingMock := &countingRegistryMockProvider{
			registryMockProvider: mock,
			listCount:            &refreshCount,
		}

		registry := NewModelRegistry()
		workingMock := &registryMockProvider{
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

		refreshCount.Store(0)

		interval := 50 * time.Millisecond
		cancel := registry.StartBackgroundRefresh(interval, "")
		defer cancel()

		time.Sleep(interval*3 + 25*time.Millisecond)

		count := refreshCount.Load()
		if count < 2 {
			t.Errorf("expected at least 2 refresh attempts despite errors, got %d", count)
		}
	})
}

// Verify ModelRegistry implements core.ModelLookup interface
var _ core.ModelLookup = (*ModelRegistry)(nil)
