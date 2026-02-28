package providers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"gomodel/internal/core"
)

// mockModelLookup implements core.ModelLookup for fast, isolated Router testing.
// This is simpler and faster than using a full ModelRegistry with providers.
type mockModelLookup struct {
	models        map[string]core.Provider
	providerTypes map[string]string
	modelList     []core.Model
}

func newMockLookup() *mockModelLookup {
	return &mockModelLookup{
		models:        make(map[string]core.Provider),
		providerTypes: make(map[string]string),
	}
}

func (m *mockModelLookup) addModel(model string, provider core.Provider, providerType string) {
	m.models[model] = provider
	m.providerTypes[model] = providerType
	m.modelList = append(m.modelList, core.Model{ID: model, Object: "model"})
}

func (m *mockModelLookup) Supports(model string) bool {
	_, ok := m.models[model]
	return ok
}

func (m *mockModelLookup) GetProvider(model string) core.Provider {
	return m.models[model]
}

func (m *mockModelLookup) GetProviderType(model string) string {
	return m.providerTypes[model]
}

func (m *mockModelLookup) ListModels() []core.Model {
	return m.modelList
}

func (m *mockModelLookup) ModelCount() int {
	return len(m.models)
}

// mockProvider is a simple mock implementation of core.Provider for testing
type mockProvider struct {
	name              string
	chatResponse      *core.ChatResponse
	responsesResponse *core.ResponsesResponse
	embeddingResponse *core.EmbeddingResponse
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
	return nil, nil
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

func (m *mockProvider) Embeddings(_ context.Context, _ *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.embeddingResponse, nil
}

type mockBatchProvider struct {
	mockProvider
}

func (m *mockBatchProvider) CreateBatch(_ context.Context, _ *core.BatchRequest) (*core.BatchResponse, error) {
	return &core.BatchResponse{ID: "provider-batch-1", Object: "batch"}, nil
}

func (m *mockBatchProvider) GetBatch(_ context.Context, _ string) (*core.BatchResponse, error) {
	return &core.BatchResponse{ID: "provider-batch-1", Object: "batch"}, nil
}

func (m *mockBatchProvider) ListBatches(_ context.Context, _ int, _ string) (*core.BatchListResponse, error) {
	return &core.BatchListResponse{Object: "list"}, nil
}

func (m *mockBatchProvider) CancelBatch(_ context.Context, _ string) (*core.BatchResponse, error) {
	return &core.BatchResponse{ID: "provider-batch-1", Object: "batch", Status: "cancelled"}, nil
}

func (m *mockBatchProvider) GetBatchResults(_ context.Context, _ string) (*core.BatchResultsResponse, error) {
	return &core.BatchResultsResponse{Object: "list", BatchID: "provider-batch-1"}, nil
}

func TestNewRouter(t *testing.T) {
	t.Run("nil lookup returns error", func(t *testing.T) {
		router, err := NewRouter(nil)
		if err == nil {
			t.Error("expected error for nil lookup")
		}
		if router != nil {
			t.Error("expected nil router")
		}
	})

	t.Run("valid lookup succeeds", func(t *testing.T) {
		lookup := newMockLookup()
		router, err := NewRouter(lookup)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if router == nil {
			t.Error("expected non-nil router")
		}
	})
}

func TestRouterEmptyLookup(t *testing.T) {
	lookup := newMockLookup() // Empty - no models
	router, _ := NewRouter(lookup)

	t.Run("Supports returns false", func(t *testing.T) {
		if router.Supports("any-model") {
			t.Error("expected false for empty lookup")
		}
	})

	t.Run("ChatCompletion returns error", func(t *testing.T) {
		_, err := router.ChatCompletion(context.Background(), &core.ChatRequest{Model: "any"})
		if !errors.Is(err, ErrRegistryNotInitialized) {
			t.Errorf("expected ErrRegistryNotInitialized, got: %v", err)
		}
	})

	t.Run("StreamChatCompletion returns error", func(t *testing.T) {
		_, err := router.StreamChatCompletion(context.Background(), &core.ChatRequest{Model: "any"})
		if !errors.Is(err, ErrRegistryNotInitialized) {
			t.Errorf("expected ErrRegistryNotInitialized, got: %v", err)
		}
	})

	t.Run("ListModels returns error", func(t *testing.T) {
		_, err := router.ListModels(context.Background())
		if !errors.Is(err, ErrRegistryNotInitialized) {
			t.Errorf("expected ErrRegistryNotInitialized, got: %v", err)
		}
	})

	t.Run("Responses returns error", func(t *testing.T) {
		_, err := router.Responses(context.Background(), &core.ResponsesRequest{Model: "any"})
		if !errors.Is(err, ErrRegistryNotInitialized) {
			t.Errorf("expected ErrRegistryNotInitialized, got: %v", err)
		}
	})

	t.Run("StreamResponses returns error", func(t *testing.T) {
		_, err := router.StreamResponses(context.Background(), &core.ResponsesRequest{Model: "any"})
		if !errors.Is(err, ErrRegistryNotInitialized) {
			t.Errorf("expected ErrRegistryNotInitialized, got: %v", err)
		}
	})
}

func TestRouterSupports(t *testing.T) {
	openai := &mockProvider{name: "openai"}
	anthropic := &mockProvider{name: "anthropic"}

	lookup := newMockLookup()
	lookup.addModel("gpt-4o", openai, "openai")
	lookup.addModel("claude-3-5-sonnet", anthropic, "anthropic")

	router, _ := NewRouter(lookup)

	tests := []struct {
		model    string
		expected bool
	}{
		{"gpt-4o", true},
		{"claude-3-5-sonnet", true},
		{"unsupported", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := router.Supports(tt.model); got != tt.expected {
				t.Errorf("Supports(%q) = %v, want %v", tt.model, got, tt.expected)
			}
		})
	}
}

func TestRouterChatCompletion(t *testing.T) {
	openaiResp := &core.ChatResponse{ID: "openai-resp", Model: "gpt-4o"}
	anthropicResp := &core.ChatResponse{ID: "anthropic-resp", Model: "claude-3-5-sonnet"}

	openai := &mockProvider{name: "openai", chatResponse: openaiResp}
	anthropic := &mockProvider{name: "anthropic", chatResponse: anthropicResp}

	lookup := newMockLookup()
	lookup.addModel("gpt-4o", openai, "openai")
	lookup.addModel("claude-3-5-sonnet", anthropic, "anthropic")

	router, _ := NewRouter(lookup)

	tests := []struct {
		name         string
		model        string
		wantResp     *core.ChatResponse
		wantProvider string
		wantError    bool
	}{
		{"routes to openai", "gpt-4o", openaiResp, "openai", false},
		{"routes to anthropic", "claude-3-5-sonnet", anthropicResp, "anthropic", false},
		{"unsupported model", "unknown", nil, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &core.ChatRequest{Model: tt.model}
			resp, err := router.ChatCompletion(context.Background(), req)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if resp.ID != tt.wantResp.ID {
				t.Errorf("got response ID %q, want %q", resp.ID, tt.wantResp.ID)
			}
			if resp.Provider != tt.wantProvider {
				t.Errorf("Provider = %q, want %q", resp.Provider, tt.wantProvider)
			}
		})
	}
}

func TestRouterResponses(t *testing.T) {
	expectedResp := &core.ResponsesResponse{ID: "resp-123"}
	provider := &mockProvider{name: "openai", responsesResponse: expectedResp}

	lookup := newMockLookup()
	lookup.addModel("gpt-4o", provider, "openai")

	router, _ := NewRouter(lookup)

	t.Run("routes correctly and stamps provider", func(t *testing.T) {
		req := &core.ResponsesRequest{Model: "gpt-4o"}
		resp, err := router.Responses(context.Background(), req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if resp.ID != expectedResp.ID {
			t.Errorf("got ID %q, want %q", resp.ID, expectedResp.ID)
		}
		if resp.Provider != "openai" {
			t.Errorf("Provider = %q, want %q", resp.Provider, "openai")
		}
	})

	t.Run("unknown model returns error", func(t *testing.T) {
		req := &core.ResponsesRequest{Model: "unknown"}
		_, err := router.Responses(context.Background(), req)
		if err == nil {
			t.Error("expected error for unknown model")
		}
	})
}

func TestRouterListModels(t *testing.T) {
	lookup := newMockLookup()
	lookup.addModel("gpt-4o", &mockProvider{}, "openai")
	lookup.addModel("claude-3-5-sonnet", &mockProvider{}, "anthropic")

	router, _ := NewRouter(lookup)

	resp, err := router.ListModels(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Data) != 2 {
		t.Errorf("expected 2 models, got %d", len(resp.Data))
	}
	if resp.Object != "list" {
		t.Errorf("expected object 'list', got %q", resp.Object)
	}
}

func TestRouterGetProviderType(t *testing.T) {
	lookup := newMockLookup()
	lookup.addModel("gpt-4o", &mockProvider{}, "openai")
	lookup.addModel("claude-3-5-sonnet", &mockProvider{}, "anthropic")

	router, _ := NewRouter(lookup)

	tests := []struct {
		model    string
		expected string
	}{
		{"gpt-4o", "openai"},
		{"claude-3-5-sonnet", "anthropic"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := router.GetProviderType(tt.model); got != tt.expected {
				t.Errorf("GetProviderType(%q) = %q, want %q", tt.model, got, tt.expected)
			}
		})
	}
}

func TestRouterBatchProviderTypeValidation(t *testing.T) {
	lookup := newMockLookup()
	lookup.addModel("gpt-4o", &mockBatchProvider{}, "openai")

	router, _ := NewRouter(lookup)

	tests := []struct {
		name         string
		providerType string
		call         func() error
	}{
		{
			name:         "empty provider type",
			providerType: "",
			call: func() error {
				_, err := router.GetBatch(context.Background(), "", "batch_1")
				return err
			},
		},
		{
			name:         "unknown provider type",
			providerType: "does-not-exist",
			call: func() error {
				_, err := router.GetBatch(context.Background(), "does-not-exist", "batch_1")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if err == nil {
				t.Fatal("expected error")
			}
			var gwErr *core.GatewayError
			if !errors.As(err, &gwErr) {
				t.Fatalf("expected GatewayError, got %T: %v", err, err)
			}
			if gwErr.HTTPStatusCode() != 400 {
				t.Fatalf("expected status 400, got %d", gwErr.HTTPStatusCode())
			}
		})
	}
}

func TestRouterEmbeddings(t *testing.T) {
	expectedResp := &core.EmbeddingResponse{
		Object:   "list",
		Model:    "text-embedding-3-small",
		Provider: "openai",
		Data: []core.EmbeddingData{
			{Object: "embedding", Embedding: json.RawMessage(`[0.1,0.2]`), Index: 0},
		},
	}
	provider := &mockProvider{name: "openai", embeddingResponse: expectedResp}

	lookup := newMockLookup()
	lookup.addModel("text-embedding-3-small", provider, "openai")

	router, _ := NewRouter(lookup)

	t.Run("routes correctly and stamps provider", func(t *testing.T) {
		req := &core.EmbeddingRequest{Model: "text-embedding-3-small", Input: "hello"}
		resp, err := router.Embeddings(context.Background(), req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if resp.Model != expectedResp.Model {
			t.Errorf("got Model %q, want %q", resp.Model, expectedResp.Model)
		}
		if resp.Provider != "openai" {
			t.Errorf("Provider = %q, want %q", resp.Provider, "openai")
		}
	})

	t.Run("unknown model returns error", func(t *testing.T) {
		req := &core.EmbeddingRequest{Model: "unknown"}
		_, err := router.Embeddings(context.Background(), req)
		if err == nil {
			t.Error("expected error for unknown model")
		}
	})
}

func TestRouterEmbeddings_EmptyLookup(t *testing.T) {
	lookup := newMockLookup()
	router, _ := NewRouter(lookup)

	_, err := router.Embeddings(context.Background(), &core.EmbeddingRequest{Model: "any"})
	if !errors.Is(err, ErrRegistryNotInitialized) {
		t.Errorf("expected ErrRegistryNotInitialized, got: %v", err)
	}
}

func TestRouterEmbeddings_ProviderError(t *testing.T) {
	providerErr := core.NewInvalidRequestError("anthropic does not support embeddings", nil)
	provider := &mockProvider{name: "anthropic", err: providerErr}

	lookup := newMockLookup()
	lookup.addModel("claude-3-5-sonnet", provider, "anthropic")

	router, _ := NewRouter(lookup)

	req := &core.EmbeddingRequest{Model: "claude-3-5-sonnet"}
	_, err := router.Embeddings(context.Background(), req)
	if err == nil {
		t.Error("expected error from provider")
	}
	var gatewayErr *core.GatewayError
	if !errors.As(err, &gatewayErr) {
		t.Errorf("expected GatewayError, got %T: %v", err, err)
	}
}

func TestRouterProviderError(t *testing.T) {
	providerErr := errors.New("provider error")
	provider := &mockProvider{name: "failing", err: providerErr}

	lookup := newMockLookup()
	lookup.addModel("failing-model", provider, "test")

	router, _ := NewRouter(lookup)

	t.Run("ChatCompletion propagates error", func(t *testing.T) {
		req := &core.ChatRequest{Model: "failing-model"}
		_, err := router.ChatCompletion(context.Background(), req)
		if !errors.Is(err, providerErr) {
			t.Errorf("expected provider error, got: %v", err)
		}
	})

	t.Run("Responses propagates error", func(t *testing.T) {
		req := &core.ResponsesRequest{Model: "failing-model"}
		_, err := router.Responses(context.Background(), req)
		if !errors.Is(err, providerErr) {
			t.Errorf("expected provider error, got: %v", err)
		}
	})
}
