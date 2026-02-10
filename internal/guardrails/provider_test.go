package guardrails

import (
	"context"
	"io"
	"strings"
	"testing"

	"gomodel/internal/core"
)

// mockRoutableProvider is a test double for core.RoutableProvider.
type mockRoutableProvider struct {
	supportsFn       func(model string) bool
	getProviderTypeFn func(model string) string
	chatReq          *core.ChatRequest
	responsesReq     *core.ResponsesRequest
}

func (m *mockRoutableProvider) Supports(model string) bool {
	if m.supportsFn != nil {
		return m.supportsFn(model)
	}
	return true
}

func (m *mockRoutableProvider) GetProviderType(model string) string {
	if m.getProviderTypeFn != nil {
		return m.getProviderTypeFn(model)
	}
	return "mock"
}

func (m *mockRoutableProvider) ChatCompletion(_ context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	m.chatReq = req
	return &core.ChatResponse{Model: req.Model}, nil
}

func (m *mockRoutableProvider) StreamChatCompletion(_ context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	m.chatReq = req
	return io.NopCloser(strings.NewReader("data: test\n\n")), nil
}

func (m *mockRoutableProvider) ListModels(_ context.Context) (*core.ModelsResponse, error) {
	return &core.ModelsResponse{Object: "list"}, nil
}

func (m *mockRoutableProvider) Responses(_ context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	m.responsesReq = req
	return &core.ResponsesResponse{Model: req.Model}, nil
}

func (m *mockRoutableProvider) StreamResponses(_ context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	m.responsesReq = req
	return io.NopCloser(strings.NewReader("data: test\n\n")), nil
}

func TestGuardedProvider_ChatCompletion_AppliesGuardrails(t *testing.T) {
	inner := &mockRoutableProvider{}
	pipeline := NewPipeline(Sequential)

	g, _ := NewSystemPromptGuardrail(SystemPromptInject, "guardrail system")
	pipeline.Add(g)

	guarded := NewGuardedProvider(inner, pipeline)

	req := &core.ChatRequest{
		Model:    "gpt-4",
		Messages: []core.Message{{Role: "user", Content: "hello"}},
	}

	_, err := guarded.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Verify the inner provider received the modified request
	if inner.chatReq == nil {
		t.Fatal("inner provider was not called")
	}
	if len(inner.chatReq.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(inner.chatReq.Messages))
	}
	if inner.chatReq.Messages[0].Role != "system" || inner.chatReq.Messages[0].Content != "guardrail system" {
		t.Errorf("expected injected system message, got %+v", inner.chatReq.Messages[0])
	}
}

func TestGuardedProvider_StreamChatCompletion_AppliesGuardrails(t *testing.T) {
	inner := &mockRoutableProvider{}
	pipeline := NewPipeline(Sequential)

	g, _ := NewSystemPromptGuardrail(SystemPromptOverride, "override system")
	pipeline.Add(g)

	guarded := NewGuardedProvider(inner, pipeline)

	req := &core.ChatRequest{
		Model: "gpt-4",
		Messages: []core.Message{
			{Role: "system", Content: "original"},
			{Role: "user", Content: "hello"},
		},
	}

	stream, err := guarded.StreamChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	if inner.chatReq.Messages[0].Content != "override system" {
		t.Errorf("expected override, got %q", inner.chatReq.Messages[0].Content)
	}
}

func TestGuardedProvider_Responses_AppliesGuardrails(t *testing.T) {
	inner := &mockRoutableProvider{}
	pipeline := NewPipeline(Sequential)

	g, _ := NewSystemPromptGuardrail(SystemPromptInject, "guardrail instructions")
	pipeline.Add(g)

	guarded := NewGuardedProvider(inner, pipeline)

	req := &core.ResponsesRequest{Model: "gpt-4", Input: "hello"}

	_, err := guarded.Responses(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if inner.responsesReq.Instructions != "guardrail instructions" {
		t.Errorf("expected injected instructions, got %q", inner.responsesReq.Instructions)
	}
}

func TestGuardedProvider_StreamResponses_AppliesGuardrails(t *testing.T) {
	inner := &mockRoutableProvider{}
	pipeline := NewPipeline(Sequential)

	g, _ := NewSystemPromptGuardrail(SystemPromptDecorator, "prefix")
	pipeline.Add(g)

	guarded := NewGuardedProvider(inner, pipeline)

	req := &core.ResponsesRequest{
		Model:        "gpt-4",
		Input:        "hello",
		Instructions: "existing",
	}

	stream, err := guarded.StreamResponses(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	if inner.responsesReq.Instructions != "prefix\nexisting" {
		t.Errorf("expected decorated instructions, got %q", inner.responsesReq.Instructions)
	}
}

func TestGuardedProvider_ListModels_NoGuardrails(t *testing.T) {
	inner := &mockRoutableProvider{}
	pipeline := NewPipeline(Sequential)
	guarded := NewGuardedProvider(inner, pipeline)

	resp, err := guarded.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Object != "list" {
		t.Errorf("expected 'list', got %q", resp.Object)
	}
}

func TestGuardedProvider_DelegatesSupports(t *testing.T) {
	inner := &mockRoutableProvider{
		supportsFn: func(model string) bool {
			return model == "gpt-4"
		},
	}
	pipeline := NewPipeline(Sequential)
	guarded := NewGuardedProvider(inner, pipeline)

	if !guarded.Supports("gpt-4") {
		t.Error("expected Supports to return true for gpt-4")
	}
	if guarded.Supports("unknown") {
		t.Error("expected Supports to return false for unknown")
	}
}

func TestGuardedProvider_DelegatesGetProviderType(t *testing.T) {
	inner := &mockRoutableProvider{
		getProviderTypeFn: func(_ string) string {
			return "openai"
		},
	}
	pipeline := NewPipeline(Sequential)
	guarded := NewGuardedProvider(inner, pipeline)

	if guarded.GetProviderType("gpt-4") != "openai" {
		t.Errorf("expected 'openai', got %q", guarded.GetProviderType("gpt-4"))
	}
}

func TestGuardedProvider_GuardrailError_BlocksRequest(t *testing.T) {
	inner := &mockRoutableProvider{}
	pipeline := NewPipeline(Sequential)
	pipeline.Add(&mockGuardrail{
		name: "blocker",
		chatFn: func(_ context.Context, _ *core.ChatRequest) (*core.ChatRequest, error) {
			return nil, core.NewInvalidRequestError("guardrail violation", nil)
		},
	})

	guarded := NewGuardedProvider(inner, pipeline)

	req := &core.ChatRequest{
		Model:    "gpt-4",
		Messages: []core.Message{{Role: "user", Content: "hello"}},
	}

	_, err := guarded.ChatCompletion(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from guardrail")
	}

	// Inner provider should not have been called
	if inner.chatReq != nil {
		t.Error("inner provider should not have been called when guardrail blocks")
	}
}
