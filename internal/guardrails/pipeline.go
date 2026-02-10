package guardrails

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"gomodel/internal/core"
)

// entry pairs a guardrail with its execution order.
type entry struct {
	guardrail Guardrail
	order     int
}

// Pipeline orchestrates the execution of multiple guardrails.
//
// Guardrails are grouped by their order value. Groups execute sequentially
// in ascending order. Within a group, all guardrails run in parallel.
// If a group contains a single guardrail, it runs directly (no goroutine overhead).
type Pipeline struct {
	entries []entry
}

// NewPipeline creates a new empty guardrails pipeline.
func NewPipeline() *Pipeline {
	return &Pipeline{}
}

// Add appends a guardrail with the given execution order.
// Guardrails with the same order run in parallel; different orders run sequentially.
func (p *Pipeline) Add(g Guardrail, order int) {
	p.entries = append(p.entries, entry{guardrail: g, order: order})
}

// Len returns the number of guardrails in the pipeline.
func (p *Pipeline) Len() int {
	return len(p.entries)
}

// groups returns entries grouped by order, sorted by ascending order value.
// Within each group, entries appear in registration order.
func (p *Pipeline) groups() [][]entry {
	if len(p.entries) == 0 {
		return nil
	}

	// Stable sort preserves registration order within same order value
	sorted := make([]entry, len(p.entries))
	copy(sorted, p.entries)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].order < sorted[j].order
	})

	var result [][]entry
	currentOrder := sorted[0].order
	currentGroup := []entry{sorted[0]}

	for i := 1; i < len(sorted); i++ {
		if sorted[i].order != currentOrder {
			result = append(result, currentGroup)
			currentOrder = sorted[i].order
			currentGroup = []entry{sorted[i]}
		} else {
			currentGroup = append(currentGroup, sorted[i])
		}
	}
	result = append(result, currentGroup)
	return result
}

// ProcessChat runs all guardrails on a ChatRequest.
func (p *Pipeline) ProcessChat(ctx context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
	groups := p.groups()
	if len(groups) == 0 {
		return req, nil
	}

	current := req
	for _, group := range groups {
		var err error
		if len(group) == 1 {
			current, err = group[0].guardrail.ProcessChat(ctx, current)
			if err != nil {
				return nil, fmt.Errorf("guardrail %q: %w", group[0].guardrail.Name(), err)
			}
		} else {
			current, err = runChatGroupParallel(ctx, group, current)
			if err != nil {
				return nil, err
			}
		}
	}
	return current, nil
}

// ProcessResponses runs all guardrails on a ResponsesRequest.
func (p *Pipeline) ProcessResponses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesRequest, error) {
	groups := p.groups()
	if len(groups) == 0 {
		return req, nil
	}

	current := req
	for _, group := range groups {
		var err error
		if len(group) == 1 {
			current, err = group[0].guardrail.ProcessResponses(ctx, current)
			if err != nil {
				return nil, fmt.Errorf("guardrail %q: %w", group[0].guardrail.Name(), err)
			}
		} else {
			current, err = runResponsesGroupParallel(ctx, group, current)
			if err != nil {
				return nil, err
			}
		}
	}
	return current, nil
}

// chatResult holds the result of a parallel guardrail execution for chat.
type chatResult struct {
	req *core.ChatRequest
	err error
}

// runChatGroupParallel runs all guardrails in a group concurrently on the same input.
// If any returns an error, the group fails. Modifications are applied
// in registration order (slice order) after all complete.
func runChatGroupParallel(ctx context.Context, group []entry, req *core.ChatRequest) (*core.ChatRequest, error) {
	results := make([]chatResult, len(group))
	var wg sync.WaitGroup

	for i, e := range group {
		wg.Add(1)
		go func(idx int, g Guardrail) {
			defer wg.Done()
			modified, err := g.ProcessChat(ctx, req)
			results[idx] = chatResult{req: modified, err: err}
		}(i, e.guardrail)
	}

	wg.Wait()

	// Check for errors and take last successful modification (registration order)
	current := req
	for i, r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("guardrail %q: %w", group[i].guardrail.Name(), r.err)
		}
		current = r.req
	}
	return current, nil
}

// responsesResult holds the result of a parallel guardrail execution for responses.
type responsesResult struct {
	req *core.ResponsesRequest
	err error
}

// runResponsesGroupParallel runs all guardrails in a group concurrently on the same input.
func runResponsesGroupParallel(ctx context.Context, group []entry, req *core.ResponsesRequest) (*core.ResponsesRequest, error) {
	results := make([]responsesResult, len(group))
	var wg sync.WaitGroup

	for i, e := range group {
		wg.Add(1)
		go func(idx int, g Guardrail) {
			defer wg.Done()
			modified, err := g.ProcessResponses(ctx, req)
			results[idx] = responsesResult{req: modified, err: err}
		}(i, e.guardrail)
	}

	wg.Wait()

	current := req
	for i, r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("guardrail %q: %w", group[i].guardrail.Name(), r.err)
		}
		current = r.req
	}
	return current, nil
}
