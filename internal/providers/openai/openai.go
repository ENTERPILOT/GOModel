// Package openai provides OpenAI API integration for the LLM gateway.
package openai

import (
	"context"
	"io"
	"net/http"

	"gomodel/internal/core"
	"gomodel/internal/llmclient"
	"gomodel/internal/providers"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
)

func init() {
	// Self-register with the factory
	providers.RegisterProvider("openai", New)
}

// Provider implements the core.Provider interface for OpenAI
type Provider struct {
	client *llmclient.Client
	apiKey string
}

// New creates a new OpenAI provider
func New(apiKey string) *Provider {
	p := &Provider{apiKey: apiKey}
	cfg := llmclient.DefaultConfig("openai", defaultBaseURL)
	// Apply global hooks if available
	cfg.Hooks = providers.GetGlobalHooks()
	p.client = llmclient.New(cfg, p.setHeaders)
	return p
}

// NewWithHTTPClient creates a new OpenAI provider with a custom HTTP client
func NewWithHTTPClient(apiKey string, httpClient *http.Client) *Provider {
	p := &Provider{apiKey: apiKey}
	cfg := llmclient.DefaultConfig("openai", defaultBaseURL)
	// Apply global hooks if available
	cfg.Hooks = providers.GetGlobalHooks()
	p.client = llmclient.NewWithHTTPClient(httpClient, cfg, p.setHeaders)
	return p
}

// SetBaseURL allows configuring a custom base URL for the provider
func (p *Provider) SetBaseURL(url string) {
	p.client.SetBaseURL(url)
}

// setHeaders sets the required headers for OpenAI API requests
func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
}

// ChatCompletion sends a chat completion request to OpenAI
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

// ListModels retrieves the list of available models from OpenAI
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

// Responses sends a Responses API request to OpenAI
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
