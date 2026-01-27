package guardrails

import (
	"io"

	"gomodel/internal/core"
)

// Processor applies guardrails to requests and responses.
type Processor struct {
	config    Config
	injector  *SystemPromptInjector
	anonymizer *Anonymizer
}

// RequestContext holds state needed for de-anonymizing responses.
// It is returned by ProcessChatRequest and should be passed to response processing.
type RequestContext struct {
	// AnonymizationMap maps tokens to original values for de-anonymization.
	AnonymizationMap map[string]string

	// Model is the request model for logging/debugging.
	Model string

	// Provider is the resolved provider type.
	Provider string
}

// New creates a new Processor with the given configuration.
func New(cfg Config) *Processor {
	p := &Processor{
		config: cfg,
	}

	if cfg.SystemPrompt.Enabled {
		p.injector = NewSystemPromptInjector(cfg.SystemPrompt)
	}

	if cfg.Anonymization.Enabled {
		p.anonymizer = NewAnonymizer(cfg.Anonymization)
	}

	return p
}

// Config returns the processor's configuration.
func (p *Processor) Config() Config {
	return p.config
}

// ProcessChatRequest applies all enabled guardrails to a ChatRequest.
// Returns the modified request and a context for response de-anonymization.
func (p *Processor) ProcessChatRequest(req *core.ChatRequest, providerType string) (*core.ChatRequest, *RequestContext) {
	if p == nil {
		return req, nil
	}

	ctx := &RequestContext{
		Model:    req.Model,
		Provider: providerType,
	}

	// Apply system prompt injection
	if p.injector != nil {
		req = p.injector.ProcessChatRequest(req, providerType)
	}

	// Apply anonymization
	if p.anonymizer != nil && p.anonymizer.ShouldAnonymize(req.Model) {
		req, ctx.AnonymizationMap = p.anonymizer.AnonymizeChatRequest(req)
	}

	return req, ctx
}

// ProcessResponsesRequest applies all enabled guardrails to a ResponsesRequest.
// Returns the modified request and a context for response de-anonymization.
func (p *Processor) ProcessResponsesRequest(req *core.ResponsesRequest, providerType string) (*core.ResponsesRequest, *RequestContext) {
	if p == nil {
		return req, nil
	}

	ctx := &RequestContext{
		Model:    req.Model,
		Provider: providerType,
	}

	// Apply system prompt injection
	if p.injector != nil {
		req = p.injector.ProcessResponsesRequest(req, providerType)
	}

	// Apply anonymization
	if p.anonymizer != nil && p.anonymizer.ShouldAnonymize(req.Model) {
		req, ctx.AnonymizationMap = p.anonymizer.AnonymizeResponsesRequest(req)
	}

	return req, ctx
}

// DeanonymizeChatResponse restores original PII values in a ChatResponse.
func (p *Processor) DeanonymizeChatResponse(resp *core.ChatResponse, ctx *RequestContext) *core.ChatResponse {
	if p == nil || ctx == nil || len(ctx.AnonymizationMap) == 0 {
		return resp
	}
	if p.anonymizer == nil || !p.config.Anonymization.DeanonymizeResponses {
		return resp
	}

	return p.anonymizer.DeanonymizeChatResponse(resp, ctx.AnonymizationMap)
}

// DeanonymizeResponsesResponse restores original PII values in a ResponsesResponse.
func (p *Processor) DeanonymizeResponsesResponse(resp *core.ResponsesResponse, ctx *RequestContext) *core.ResponsesResponse {
	if p == nil || ctx == nil || len(ctx.AnonymizationMap) == 0 {
		return resp
	}
	if p.anonymizer == nil || !p.config.Anonymization.DeanonymizeResponses {
		return resp
	}

	return p.anonymizer.DeanonymizeResponsesResponse(resp, ctx.AnonymizationMap)
}

// WrapStreamForDeanonymization wraps a streaming response for de-anonymization.
// Returns the original stream if de-anonymization is not needed.
func (p *Processor) WrapStreamForDeanonymization(stream io.ReadCloser, ctx *RequestContext) io.ReadCloser {
	if p == nil || ctx == nil || len(ctx.AnonymizationMap) == 0 {
		return stream
	}
	if p.anonymizer == nil || !p.config.Anonymization.DeanonymizeResponses {
		return stream
	}

	return NewDeanonymizingReader(stream, ctx.AnonymizationMap)
}
