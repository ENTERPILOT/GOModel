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
	name           string
	supportedModel string
	chatResponse   *core.ChatResponse
	modelsResponse *core.ModelsResponse
	err            error
}

func (m *mockProvider) Supports(model string) bool {
	return model == m.supportedModel
}

func (m *mockProvider) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.chatResponse, nil
}

func (m *mockProvider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	return io.NopCloser(nil), nil
}

func (m *mockProvider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.modelsResponse, nil
}

func TestRouterSupports(t *testing.T) {
	openaiMock := &mockProvider{name: "openai", supportedModel: "gpt-4o"}
	anthropicMock := &mockProvider{name: "anthropic", supportedModel: "claude-3-5-sonnet-20241022"}

	router := NewRouter(openaiMock, anthropicMock)

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
		name:           "openai",
		supportedModel: "gpt-4o",
		chatResponse:   openaiResp,
	}
	anthropicMock := &mockProvider{
		name:           "anthropic",
		supportedModel: "claude-3-5-sonnet-20241022",
		chatResponse:   anthropicResp,
	}

	router := NewRouter(openaiMock, anthropicMock)

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
	openaiModels := &core.ModelsResponse{
		Object: "list",
		Data: []core.Model{
			{ID: "gpt-4o", Object: "model", OwnedBy: "openai"},
		},
	}
	anthropicModels := &core.ModelsResponse{
		Object: "list",
		Data: []core.Model{
			{ID: "claude-3-5-sonnet-20241022", Object: "model", OwnedBy: "anthropic"},
		},
	}

	openaiMock := &mockProvider{
		name:           "openai",
		supportedModel: "gpt-4o",
		modelsResponse: openaiModels,
	}
	anthropicMock := &mockProvider{
		name:           "anthropic",
		supportedModel: "claude-3-5-sonnet-20241022",
		modelsResponse: anthropicModels,
	}

	router := NewRouter(openaiMock, anthropicMock)

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
	// Test that router continues even if one provider fails
	openaiMock := &mockProvider{
		name:           "openai",
		supportedModel: "gpt-4o",
		modelsResponse: &core.ModelsResponse{
			Object: "list",
			Data: []core.Model{
				{ID: "gpt-4o", Object: "model", OwnedBy: "openai"},
			},
		},
	}
	anthropicMock := &mockProvider{
		name:           "anthropic",
		supportedModel: "claude-3-5-sonnet-20241022",
		err:            errors.New("provider error"),
	}

	router := NewRouter(openaiMock, anthropicMock)

	resp, err := router.ListModels(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still get OpenAI models even though Anthropic failed
	if len(resp.Data) != 1 {
		t.Errorf("expected 1 model, got %d", len(resp.Data))
	}
	if resp.Data[0].ID != "gpt-4o" {
		t.Errorf("expected gpt-4o, got %s", resp.Data[0].ID)
	}
}


