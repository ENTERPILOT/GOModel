package guardrails

import (
	"context"
	"fmt"
	"sync"

	"gomodel/internal/core"
)

// Pipeline orchestrates the execution of multiple guardrails.
type Pipeline struct {
	guardrails []Guardrail
	mode       ExecutionMode
}

// NewPipeline creates a new guardrails pipeline with the given execution mode.
func NewPipeline(mode ExecutionMode) *Pipeline {
	if mode != Parallel {
		mode = Sequential // default to sequential
	}
	return &Pipeline{
		mode: mode,
	}
}

// Add appends a guardrail to the pipeline.
func (p *Pipeline) Add(g Guardrail) {
	p.guardrails = append(p.guardrails, g)
}

// Len returns the number of guardrails in the pipeline.
func (p *Pipeline) Len() int {
	return len(p.guardrails)
}

// ProcessChat runs all guardrails on a ChatRequest.
func (p *Pipeline) ProcessChat(ctx context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
	if len(p.guardrails) == 0 {
		return req, nil
	}

	if p.mode == Parallel {
		return p.processChatParallel(ctx, req)
	}
	return p.processChatSequential(ctx, req)
}

// ProcessResponses runs all guardrails on a ResponsesRequest.
func (p *Pipeline) ProcessResponses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesRequest, error) {
	if len(p.guardrails) == 0 {
		return req, nil
	}

	if p.mode == Parallel {
		return p.processResponsesParallel(ctx, req)
	}
	return p.processResponsesSequential(ctx, req)
}

// processChatSequential chains guardrails: each receives the output of the previous.
func (p *Pipeline) processChatSequential(ctx context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
	current := req
	for _, g := range p.guardrails {
		var err error
		current, err = g.ProcessChat(ctx, current)
		if err != nil {
			return nil, fmt.Errorf("guardrail %q: %w", g.Name(), err)
		}
	}
	return current, nil
}

// processResponsesSequential chains guardrails: each receives the output of the previous.
func (p *Pipeline) processResponsesSequential(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesRequest, error) {
	current := req
	for _, g := range p.guardrails {
		var err error
		current, err = g.ProcessResponses(ctx, current)
		if err != nil {
			return nil, fmt.Errorf("guardrail %q: %w", g.Name(), err)
		}
	}
	return current, nil
}

// chatResult holds the result of a parallel guardrail execution for chat.
type chatResult struct {
	index int
	req   *core.ChatRequest
	err   error
}

// processChatParallel runs all guardrails concurrently on the original request.
// If any returns an error, the pipeline fails. Modifications are applied
// sequentially in registration order after all guardrails complete.
func (p *Pipeline) processChatParallel(ctx context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
	results := make([]chatResult, len(p.guardrails))
	var wg sync.WaitGroup

	for i, g := range p.guardrails {
		wg.Add(1)
		go func(idx int, guardrail Guardrail) {
			defer wg.Done()
			modified, err := guardrail.ProcessChat(ctx, req)
			results[idx] = chatResult{index: idx, req: modified, err: err}
		}(i, g)
	}

	wg.Wait()

	// Check for errors and apply modifications in order
	current := req
	for i, r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("guardrail %q: %w", p.guardrails[i].Name(), r.err)
		}
		current = r.req
	}
	return current, nil
}

// responsesResult holds the result of a parallel guardrail execution for responses.
type responsesResult struct {
	index int
	req   *core.ResponsesRequest
	err   error
}

// processResponsesParallel runs all guardrails concurrently on the original request.
func (p *Pipeline) processResponsesParallel(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesRequest, error) {
	results := make([]responsesResult, len(p.guardrails))
	var wg sync.WaitGroup

	for i, g := range p.guardrails {
		wg.Add(1)
		go func(idx int, guardrail Guardrail) {
			defer wg.Done()
			modified, err := guardrail.ProcessResponses(ctx, req)
			results[idx] = responsesResult{index: idx, req: modified, err: err}
		}(i, g)
	}

	wg.Wait()

	// Check for errors and apply modifications in order
	current := req
	for i, r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("guardrail %q: %w", p.guardrails[i].Name(), r.err)
		}
		current = r.req
	}
	return current, nil
}
