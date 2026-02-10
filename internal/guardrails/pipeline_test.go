package guardrails

import (
	"context"
	"fmt"
	"testing"

	"gomodel/internal/core"
)

// mockGuardrail is a test guardrail that can be configured to modify or reject requests.
type mockGuardrail struct {
	name       string
	chatFn     func(ctx context.Context, req *core.ChatRequest) (*core.ChatRequest, error)
	responsesFn func(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesRequest, error)
}

func (m *mockGuardrail) Name() string { return m.name }

func (m *mockGuardrail) ProcessChat(ctx context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
	if m.chatFn != nil {
		return m.chatFn(ctx, req)
	}
	return req, nil
}

func (m *mockGuardrail) ProcessResponses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesRequest, error) {
	if m.responsesFn != nil {
		return m.responsesFn(ctx, req)
	}
	return req, nil
}

func TestPipeline_EmptyPipeline(t *testing.T) {
	p := NewPipeline(Sequential)
	req := &core.ChatRequest{Model: "gpt-4"}

	result, err := p.ProcessChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result != req {
		t.Error("empty pipeline should return the same request")
	}
}

func TestPipeline_Len(t *testing.T) {
	p := NewPipeline(Sequential)
	if p.Len() != 0 {
		t.Errorf("expected 0, got %d", p.Len())
	}

	p.Add(&mockGuardrail{name: "a"})
	p.Add(&mockGuardrail{name: "b"})
	if p.Len() != 2 {
		t.Errorf("expected 2, got %d", p.Len())
	}
}

func TestPipeline_Sequential_Chaining(t *testing.T) {
	p := NewPipeline(Sequential)

	// First guardrail adds a message
	p.Add(&mockGuardrail{
		name: "add_system",
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
			msgs := make([]core.Message, 0, len(req.Messages)+1)
			msgs = append(msgs, core.Message{Role: "system", Content: "first"})
			msgs = append(msgs, req.Messages...)
			return &core.ChatRequest{Model: req.Model, Messages: msgs}, nil
		},
	})

	// Second guardrail appends to messages
	p.Add(&mockGuardrail{
		name: "add_context",
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
			msgs := make([]core.Message, len(req.Messages))
			copy(msgs, req.Messages)
			msgs = append(msgs, core.Message{Role: "system", Content: "second"})
			return &core.ChatRequest{Model: req.Model, Messages: msgs}, nil
		},
	})

	req := &core.ChatRequest{
		Model:    "gpt-4",
		Messages: []core.Message{{Role: "user", Content: "hello"}},
	}

	result, err := p.ProcessChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Sequential: first adds system at start, second adds system at end
	if len(result.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result.Messages))
	}
	if result.Messages[0].Content != "first" {
		t.Errorf("expected first guardrail output, got %q", result.Messages[0].Content)
	}
	if result.Messages[2].Content != "second" {
		t.Errorf("expected second guardrail output, got %q", result.Messages[2].Content)
	}
}

func TestPipeline_Sequential_ErrorStopsChain(t *testing.T) {
	p := NewPipeline(Sequential)

	p.Add(&mockGuardrail{
		name: "blocker",
		chatFn: func(_ context.Context, _ *core.ChatRequest) (*core.ChatRequest, error) {
			return nil, fmt.Errorf("blocked")
		},
	})

	// This should never run
	called := false
	p.Add(&mockGuardrail{
		name: "second",
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
			called = true
			return req, nil
		},
	})

	req := &core.ChatRequest{Model: "gpt-4"}
	_, err := p.ProcessChat(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
	if called {
		t.Error("second guardrail should not have been called")
	}
}

func TestPipeline_Parallel_AllPass(t *testing.T) {
	p := NewPipeline(Parallel)

	p.Add(&mockGuardrail{
		name: "passthrough1",
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
			return req, nil
		},
	})
	p.Add(&mockGuardrail{
		name: "passthrough2",
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
			return req, nil
		},
	})

	req := &core.ChatRequest{Model: "gpt-4"}
	result, err := p.ProcessChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Model != "gpt-4" {
		t.Error("model should be preserved")
	}
}

func TestPipeline_Parallel_OneErrors(t *testing.T) {
	p := NewPipeline(Parallel)

	p.Add(&mockGuardrail{
		name: "pass",
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
			return req, nil
		},
	})
	p.Add(&mockGuardrail{
		name: "blocker",
		chatFn: func(_ context.Context, _ *core.ChatRequest) (*core.ChatRequest, error) {
			return nil, fmt.Errorf("blocked")
		},
	})

	req := &core.ChatRequest{Model: "gpt-4"}
	_, err := p.ProcessChat(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from parallel pipeline")
	}
}

func TestPipeline_DefaultsToSequential(t *testing.T) {
	p := NewPipeline("invalid")
	if p.mode != Sequential {
		t.Errorf("expected default sequential mode, got %q", p.mode)
	}
}

func TestPipeline_Responses_Sequential(t *testing.T) {
	p := NewPipeline(Sequential)

	p.Add(&mockGuardrail{
		name: "set_instructions",
		responsesFn: func(_ context.Context, req *core.ResponsesRequest) (*core.ResponsesRequest, error) {
			return &core.ResponsesRequest{
				Model:        req.Model,
				Input:        req.Input,
				Instructions: "from guardrail",
			}, nil
		},
	})

	req := &core.ResponsesRequest{Model: "gpt-4", Input: "hello"}
	result, err := p.ProcessResponses(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Instructions != "from guardrail" {
		t.Errorf("expected instructions from guardrail, got %q", result.Instructions)
	}
}

func TestPipeline_Responses_Parallel_OneErrors(t *testing.T) {
	p := NewPipeline(Parallel)

	p.Add(&mockGuardrail{
		name: "pass",
		responsesFn: func(_ context.Context, req *core.ResponsesRequest) (*core.ResponsesRequest, error) {
			return req, nil
		},
	})
	p.Add(&mockGuardrail{
		name: "blocker",
		responsesFn: func(_ context.Context, _ *core.ResponsesRequest) (*core.ResponsesRequest, error) {
			return nil, fmt.Errorf("blocked")
		},
	})

	req := &core.ResponsesRequest{Model: "gpt-4", Input: "hello"}
	_, err := p.ProcessResponses(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from parallel pipeline")
	}
}
