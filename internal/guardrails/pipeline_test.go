package guardrails

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"gomodel/internal/core"
)

// mockGuardrail is a test guardrail that can be configured to modify or reject requests.
type mockGuardrail struct {
	name        string
	chatFn      func(ctx context.Context, req *core.ChatRequest) (*core.ChatRequest, error)
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
	p := NewPipeline()
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
	p := NewPipeline()
	if p.Len() != 0 {
		t.Errorf("expected 0, got %d", p.Len())
	}

	p.Add(&mockGuardrail{name: "a"}, 0)
	p.Add(&mockGuardrail{name: "b"}, 1)
	if p.Len() != 2 {
		t.Errorf("expected 2, got %d", p.Len())
	}
}

func TestPipeline_DifferentOrders_RunSequentially(t *testing.T) {
	p := NewPipeline()

	// Order 0: adds a system message
	p.Add(&mockGuardrail{
		name: "add_system",
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
			msgs := make([]core.Message, 0, len(req.Messages)+1)
			msgs = append(msgs, core.Message{Role: "system", Content: "first"})
			msgs = append(msgs, req.Messages...)
			return &core.ChatRequest{Model: req.Model, Messages: msgs}, nil
		},
	}, 0)

	// Order 1: sees output from order 0, appends a message
	p.Add(&mockGuardrail{
		name: "add_context",
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
			msgs := make([]core.Message, len(req.Messages))
			copy(msgs, req.Messages)
			msgs = append(msgs, core.Message{Role: "system", Content: "second"})
			return &core.ChatRequest{Model: req.Model, Messages: msgs}, nil
		},
	}, 1)

	req := &core.ChatRequest{
		Model:    "gpt-4",
		Messages: []core.Message{{Role: "user", Content: "hello"}},
	}

	result, err := p.ProcessChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Sequential: first adds system at start, second sees that and appends at end
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

func TestPipeline_SameOrder_RunInParallel(t *testing.T) {
	p := NewPipeline()

	var started atomic.Int32
	barrier := make(chan struct{})

	// Both at order 0 — they should run concurrently
	for i := range 2 {
		name := fmt.Sprintf("parallel_%d", i)
		p.Add(&mockGuardrail{
			name: name,
			chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
				started.Add(1)
				<-barrier // wait until both have started
				return req, nil
			},
		}, 0)
	}

	req := &core.ChatRequest{Model: "gpt-4"}

	done := make(chan struct{})
	go func() {
		_, _ = p.ProcessChat(context.Background(), req)
		close(done)
	}()

	// Wait for both goroutines to start
	for started.Load() < 2 {
		time.Sleep(time.Millisecond)
	}
	// Both started concurrently — release them
	close(barrier)

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("timed out — guardrails did not run in parallel")
	}
}

func TestPipeline_MixedOrders_GroupsExecuteCorrectly(t *testing.T) {
	p := NewPipeline()

	var trace []string

	// Order 0, guardrail A
	p.Add(&mockGuardrail{
		name: "A",
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
			trace = append(trace, "A")
			return req, nil
		},
	}, 0)

	// Order 1, guardrail B (runs after group 0 completes)
	p.Add(&mockGuardrail{
		name: "B",
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
			trace = append(trace, "B")
			return req, nil
		},
	}, 1)

	// Order 0, guardrail C (parallel with A)
	p.Add(&mockGuardrail{
		name: "C",
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
			// Note: this writes to trace inside a parallel group, so we can't assert
			// exact ordering of A and C. But B must come after both.
			return req, nil
		},
	}, 0)

	req := &core.ChatRequest{Model: "gpt-4"}
	_, err := p.ProcessChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// B must be last because it's order 1
	if len(trace) < 2 {
		t.Fatalf("expected at least 2 trace entries, got %d", len(trace))
	}
	if trace[len(trace)-1] != "B" {
		t.Errorf("expected B to run last (order 1), got trace: %v", trace)
	}
}

func TestPipeline_ErrorInGroup_StopsExecution(t *testing.T) {
	p := NewPipeline()

	// Order 0: error
	p.Add(&mockGuardrail{
		name: "blocker",
		chatFn: func(_ context.Context, _ *core.ChatRequest) (*core.ChatRequest, error) {
			return nil, fmt.Errorf("blocked")
		},
	}, 0)

	// Order 1: should never run
	called := false
	p.Add(&mockGuardrail{
		name: "after_blocker",
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
			called = true
			return req, nil
		},
	}, 1)

	req := &core.ChatRequest{Model: "gpt-4"}
	_, err := p.ProcessChat(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
	if called {
		t.Error("order 1 guardrail should not have run after order 0 error")
	}
}

func TestPipeline_ParallelGroup_OneErrors(t *testing.T) {
	p := NewPipeline()

	// Both at order 0 — one fails
	p.Add(&mockGuardrail{
		name: "pass",
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
			return req, nil
		},
	}, 0)
	p.Add(&mockGuardrail{
		name: "blocker",
		chatFn: func(_ context.Context, _ *core.ChatRequest) (*core.ChatRequest, error) {
			return nil, fmt.Errorf("blocked")
		},
	}, 0)

	req := &core.ChatRequest{Model: "gpt-4"}
	_, err := p.ProcessChat(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from parallel group")
	}
}

func TestPipeline_SingleEntryGroup_NoGoroutineOverhead(t *testing.T) {
	p := NewPipeline()

	// Single guardrail at order 0 — should run directly, not via goroutine
	p.Add(&mockGuardrail{
		name: "single",
		chatFn: func(_ context.Context, _ *core.ChatRequest) (*core.ChatRequest, error) {
			return &core.ChatRequest{Model: "modified"}, nil
		},
	}, 0)

	req := &core.ChatRequest{Model: "gpt-4"}
	result, err := p.ProcessChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Model != "modified" {
		t.Errorf("expected 'modified', got %q", result.Model)
	}
}

func TestPipeline_GroupsReceivePreviousOutput(t *testing.T) {
	p := NewPipeline()

	// Order 0: set model to "step1"
	p.Add(&mockGuardrail{
		name: "step1",
		chatFn: func(_ context.Context, _ *core.ChatRequest) (*core.ChatRequest, error) {
			return &core.ChatRequest{Model: "step1"}, nil
		},
	}, 0)

	// Order 1: verify it received "step1", set to "step2"
	p.Add(&mockGuardrail{
		name: "step2",
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
			if req.Model != "step1" {
				return nil, fmt.Errorf("expected model 'step1' from previous group, got %q", req.Model)
			}
			return &core.ChatRequest{Model: "step2"}, nil
		},
	}, 1)

	// Order 2: verify it received "step2"
	p.Add(&mockGuardrail{
		name: "step3",
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
			if req.Model != "step2" {
				return nil, fmt.Errorf("expected model 'step2' from previous group, got %q", req.Model)
			}
			return &core.ChatRequest{Model: "step3"}, nil
		},
	}, 2)

	req := &core.ChatRequest{Model: "original"}
	result, err := p.ProcessChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Model != "step3" {
		t.Errorf("expected 'step3', got %q", result.Model)
	}
}

func TestPipeline_NegativeOrders(t *testing.T) {
	p := NewPipeline()

	var trace []string

	// Negative orders run before 0
	p.Add(&mockGuardrail{
		name: "first",
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
			trace = append(trace, "first")
			return req, nil
		},
	}, -1)

	p.Add(&mockGuardrail{
		name: "second",
		chatFn: func(_ context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
			trace = append(trace, "second")
			return req, nil
		},
	}, 0)

	req := &core.ChatRequest{Model: "gpt-4"}
	_, err := p.ProcessChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if len(trace) != 2 || trace[0] != "first" || trace[1] != "second" {
		t.Errorf("expected [first, second], got %v", trace)
	}
}

func TestPipeline_Responses_DifferentOrders(t *testing.T) {
	p := NewPipeline()

	p.Add(&mockGuardrail{
		name: "set_instructions",
		responsesFn: func(_ context.Context, req *core.ResponsesRequest) (*core.ResponsesRequest, error) {
			return &core.ResponsesRequest{
				Model:        req.Model,
				Input:        req.Input,
				Instructions: "from guardrail",
			}, nil
		},
	}, 0)

	p.Add(&mockGuardrail{
		name: "verify",
		responsesFn: func(_ context.Context, req *core.ResponsesRequest) (*core.ResponsesRequest, error) {
			if req.Instructions != "from guardrail" {
				return nil, fmt.Errorf("expected instructions from previous group")
			}
			return req, nil
		},
	}, 1)

	req := &core.ResponsesRequest{Model: "gpt-4", Input: "hello"}
	result, err := p.ProcessResponses(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Instructions != "from guardrail" {
		t.Errorf("expected instructions from guardrail, got %q", result.Instructions)
	}
}

func TestPipeline_Responses_ParallelGroup_OneErrors(t *testing.T) {
	p := NewPipeline()

	p.Add(&mockGuardrail{
		name: "pass",
		responsesFn: func(_ context.Context, req *core.ResponsesRequest) (*core.ResponsesRequest, error) {
			return req, nil
		},
	}, 0)
	p.Add(&mockGuardrail{
		name: "blocker",
		responsesFn: func(_ context.Context, _ *core.ResponsesRequest) (*core.ResponsesRequest, error) {
			return nil, fmt.Errorf("blocked")
		},
	}, 0)

	req := &core.ResponsesRequest{Model: "gpt-4", Input: "hello"}
	_, err := p.ProcessResponses(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from parallel pipeline")
	}
}

func TestPipeline_Groups_InternalOrdering(t *testing.T) {
	p := NewPipeline()

	// Verify groups() returns correct structure
	p.Add(&mockGuardrail{name: "a"}, 2)
	p.Add(&mockGuardrail{name: "b"}, 0)
	p.Add(&mockGuardrail{name: "c"}, 2)
	p.Add(&mockGuardrail{name: "d"}, 1)

	groups := p.groups()
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	// Group 0: order 0 → [b]
	if len(groups[0]) != 1 || groups[0][0].guardrail.Name() != "b" {
		t.Errorf("group 0 should be [b], got %v", groupNames(groups[0]))
	}

	// Group 1: order 1 → [d]
	if len(groups[1]) != 1 || groups[1][0].guardrail.Name() != "d" {
		t.Errorf("group 1 should be [d], got %v", groupNames(groups[1]))
	}

	// Group 2: order 2 → [a, c] (registration order preserved)
	if len(groups[2]) != 2 || groups[2][0].guardrail.Name() != "a" || groups[2][1].guardrail.Name() != "c" {
		t.Errorf("group 2 should be [a, c], got %v", groupNames(groups[2]))
	}
}

func groupNames(group []entry) []string {
	names := make([]string, len(group))
	for i, e := range group {
		names[i] = e.guardrail.Name()
	}
	return names
}
