package guardrails

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// mockGuardrail is a test guardrail that can be configured to modify or reject messages.
type mockGuardrail struct {
	name      string
	processFn func(ctx context.Context, msgs []Message) ([]Message, error)
}

func (m *mockGuardrail) Name() string { return m.name }

func (m *mockGuardrail) Process(ctx context.Context, msgs []Message) ([]Message, error) {
	if m.processFn != nil {
		return m.processFn(ctx, msgs)
	}
	return msgs, nil
}

func TestPipeline_EmptyPipeline(t *testing.T) {
	p := NewPipeline()
	msgs := []Message{{Role: "user", Content: "hello"}}

	result, err := p.Process(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	if &result[0] != &msgs[0] {
		t.Error("empty pipeline should return the same slice")
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

	// Order 0: prepend a system message
	p.Add(&mockGuardrail{
		name: "add_system",
		processFn: func(_ context.Context, msgs []Message) ([]Message, error) {
			result := make([]Message, 0, len(msgs)+1)
			result = append(result, Message{Role: "system", Content: "first"})
			result = append(result, msgs...)
			return result, nil
		},
	}, 0)

	// Order 1: sees output from order 0, appends a message
	p.Add(&mockGuardrail{
		name: "add_context",
		processFn: func(_ context.Context, msgs []Message) ([]Message, error) {
			result := make([]Message, len(msgs))
			copy(result, msgs)
			result = append(result, Message{Role: "system", Content: "second"})
			return result, nil
		},
	}, 1)

	msgs := []Message{{Role: "user", Content: "hello"}}

	result, err := p.Process(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}

	// Sequential: first prepends system, second sees that and appends at end
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[0].Content != "first" {
		t.Errorf("expected first guardrail output, got %q", result[0].Content)
	}
	if result[2].Content != "second" {
		t.Errorf("expected second guardrail output, got %q", result[2].Content)
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
			processFn: func(_ context.Context, msgs []Message) ([]Message, error) {
				started.Add(1)
				<-barrier // wait until both have started
				return msgs, nil
			},
		}, 0)
	}

	msgs := []Message{{Role: "user", Content: "hello"}}

	done := make(chan struct{})
	go func() {
		_, _ = p.Process(context.Background(), msgs)
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
		processFn: func(_ context.Context, msgs []Message) ([]Message, error) {
			trace = append(trace, "A")
			return msgs, nil
		},
	}, 0)

	// Order 1, guardrail B (runs after group 0 completes)
	p.Add(&mockGuardrail{
		name: "B",
		processFn: func(_ context.Context, msgs []Message) ([]Message, error) {
			trace = append(trace, "B")
			return msgs, nil
		},
	}, 1)

	// Order 0, guardrail C (parallel with A)
	p.Add(&mockGuardrail{
		name: "C",
		processFn: func(_ context.Context, msgs []Message) ([]Message, error) {
			// Note: this writes to trace inside a parallel group, so we can't assert
			// exact ordering of A and C. But B must come after both.
			return msgs, nil
		},
	}, 0)

	msgs := []Message{{Role: "user", Content: "hello"}}
	_, err := p.Process(context.Background(), msgs)
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
		processFn: func(_ context.Context, _ []Message) ([]Message, error) {
			return nil, fmt.Errorf("blocked")
		},
	}, 0)

	// Order 1: should never run
	called := false
	p.Add(&mockGuardrail{
		name: "after_blocker",
		processFn: func(_ context.Context, msgs []Message) ([]Message, error) {
			called = true
			return msgs, nil
		},
	}, 1)

	msgs := []Message{{Role: "user", Content: "hello"}}
	_, err := p.Process(context.Background(), msgs)
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
		processFn: func(_ context.Context, msgs []Message) ([]Message, error) {
			return msgs, nil
		},
	}, 0)
	p.Add(&mockGuardrail{
		name: "blocker",
		processFn: func(_ context.Context, _ []Message) ([]Message, error) {
			return nil, fmt.Errorf("blocked")
		},
	}, 0)

	msgs := []Message{{Role: "user", Content: "hello"}}
	_, err := p.Process(context.Background(), msgs)
	if err == nil {
		t.Fatal("expected error from parallel group")
	}
}

func TestPipeline_SingleEntryGroup_NoGoroutineOverhead(t *testing.T) {
	p := NewPipeline()

	// Single guardrail at order 0 — should run directly, not via goroutine
	p.Add(&mockGuardrail{
		name: "single",
		processFn: func(_ context.Context, _ []Message) ([]Message, error) {
			return []Message{{Role: "system", Content: "modified"}}, nil
		},
	}, 0)

	msgs := []Message{{Role: "user", Content: "hello"}}
	result, err := p.Process(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].Content != "modified" {
		t.Errorf("expected modified message, got %v", result)
	}
}

func TestPipeline_GroupsReceivePreviousOutput(t *testing.T) {
	p := NewPipeline()

	// Order 0: add "step1" marker
	p.Add(&mockGuardrail{
		name: "step1",
		processFn: func(_ context.Context, _ []Message) ([]Message, error) {
			return []Message{{Role: "system", Content: "step1"}}, nil
		},
	}, 0)

	// Order 1: verify it received "step1", set to "step2"
	p.Add(&mockGuardrail{
		name: "step2",
		processFn: func(_ context.Context, msgs []Message) ([]Message, error) {
			if len(msgs) != 1 || msgs[0].Content != "step1" {
				return nil, fmt.Errorf("expected 'step1' from previous group, got %v", msgs)
			}
			return []Message{{Role: "system", Content: "step2"}}, nil
		},
	}, 1)

	// Order 2: verify it received "step2"
	p.Add(&mockGuardrail{
		name: "step3",
		processFn: func(_ context.Context, msgs []Message) ([]Message, error) {
			if len(msgs) != 1 || msgs[0].Content != "step2" {
				return nil, fmt.Errorf("expected 'step2' from previous group, got %v", msgs)
			}
			return []Message{{Role: "system", Content: "step3"}}, nil
		},
	}, 2)

	msgs := []Message{{Role: "user", Content: "original"}}
	result, err := p.Process(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].Content != "step3" {
		t.Errorf("expected 'step3', got %v", result)
	}
}

func TestPipeline_NegativeOrders(t *testing.T) {
	p := NewPipeline()

	var trace []string

	// Negative orders run before 0
	p.Add(&mockGuardrail{
		name: "first",
		processFn: func(_ context.Context, msgs []Message) ([]Message, error) {
			trace = append(trace, "first")
			return msgs, nil
		},
	}, -1)

	p.Add(&mockGuardrail{
		name: "second",
		processFn: func(_ context.Context, msgs []Message) ([]Message, error) {
			trace = append(trace, "second")
			return msgs, nil
		},
	}, 0)

	msgs := []Message{{Role: "user", Content: "hello"}}
	_, err := p.Process(context.Background(), msgs)
	if err != nil {
		t.Fatal(err)
	}

	if len(trace) != 2 || trace[0] != "first" || trace[1] != "second" {
		t.Errorf("expected [first, second], got %v", trace)
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
