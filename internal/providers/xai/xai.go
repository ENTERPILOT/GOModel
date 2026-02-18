// Package xai provides xAI (Grok) API integration for the LLM gateway.
package xai

import (
	"context"
	"io"
	"net/http"
	"time"

	"gomodel/internal/core"
	"gomodel/internal/llmclient"
	"gomodel/internal/providers"
)

// Registration provides factory registration for the xAI provider.
var Registration = providers.Registration{
	Type: "xai",
	New:  New,
}

const (
	defaultBaseURL = "https://api.x.ai/v1"
)

// Provider implements the core.Provider interface for xAI
type Provider struct {
	client *llmclient.Client
	apiKey string
}

// New creates a new xAI provider.
func New(apiKey string, opts providers.ProviderOptions) core.Provider {
	p := &Provider{apiKey: apiKey}
	cfg := llmclient.Config{
		ProviderName: "xai",
		BaseURL:      defaultBaseURL,
		Retry:        opts.Resilience.Retry,
		Hooks:        opts.Hooks,
		CircuitBreaker: &llmclient.CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 2,
			Timeout:          30 * time.Second,
		},
	}
	p.client = llmclient.New(cfg, p.setHeaders)
	return p
}

// NewWithHTTPClient creates a new xAI provider with a custom HTTP client.
// If httpClient is nil, http.DefaultClient is used.
func NewWithHTTPClient(apiKey string, httpClient *http.Client, hooks llmclient.Hooks) *Provider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	p := &Provider{apiKey: apiKey}
	cfg := llmclient.DefaultConfig("xai", defaultBaseURL)
	cfg.Hooks = hooks
	p.client = llmclient.NewWithHTTPClient(httpClient, cfg, p.setHeaders)
	return p
}

// SetBaseURL allows configuring a custom base URL for the provider
func (p *Provider) SetBaseURL(url string) {
	p.client.SetBaseURL(url)
}

// setHeaders sets the required headers for xAI API requests
func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	// Forward request ID if present in context
	if requestID := core.GetRequestID(req.Context()); requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}
}

// ChatCompletion sends a chat completion request to xAI
func (p *Provider) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	var resp core.ChatResponse
	err := p.client.Do(ctx, llmclient.Request{
		Method:   http.MethodPost,
		Endpoint: "/chat/completions",
		Body:     req,
	}, &resp)
	if err != nil {
		return nil, err
	}
	resp.Provider = "xai"
	if resp.Model == "" {
		resp.Model = req.Model
	}
	return &resp, nil
}

// StreamChatCompletion returns a raw response body for streaming (caller must close)
func (p *Provider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	return p.client.DoStream(ctx, llmclient.Request{
		Method:   http.MethodPost,
		Endpoint: "/chat/completions",
		Body:     req.WithStreaming(),
	})
}

// ListModels retrieves the list of available models from xAI
func (p *Provider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	var resp core.ModelsResponse
	err := p.client.Do(ctx, llmclient.Request{
		Method:   http.MethodGet,
		Endpoint: "/models",
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// Responses sends a Responses API request to xAI
func (p *Provider) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	var resp core.ResponsesResponse
	err := p.client.Do(ctx, llmclient.Request{
		Method:   http.MethodPost,
		Endpoint: "/responses",
		Body:     req,
	}, &resp)
	if err != nil {
		return nil, err
	}
	resp.Provider = "xai"
	if resp.Model == "" {
		resp.Model = req.Model
	}
	return &resp, nil
}

// StreamResponses returns a raw response body for streaming Responses API (caller must close)
func (p *Provider) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	return p.client.DoStream(ctx, llmclient.Request{
		Method:   http.MethodPost,
		Endpoint: "/responses",
		Body:     req.WithStreaming(),
	})
}
