// Package openai provides OpenAI API integration for the LLM gateway.
package openai

import (
	"context"
	"io"
	"net/http"
	"strings"

	"gomodel/internal/core"
	"gomodel/internal/llmclient"
	"gomodel/internal/providers"
)

// Registration provides factory registration for the OpenAI provider.
var Registration = providers.Registration{
	Type: "openai",
	New:  New,
}

const (
	defaultBaseURL = "https://api.openai.com/v1"
)

// Provider implements the core.Provider interface for OpenAI
type Provider struct {
	client *llmclient.Client
	apiKey string
}

// New creates a new OpenAI provider.
func New(apiKey string, opts providers.ProviderOptions) core.Provider {
	p := &Provider{apiKey: apiKey}
	cfg := llmclient.Config{
		ProviderName: "openai",
		BaseURL:      defaultBaseURL,
		Retry:        opts.Resilience.Retry,
		Hooks:        opts.Hooks,
		CircuitBreaker: opts.Resilience.CircuitBreaker,
	}
	p.client = llmclient.New(cfg, p.setHeaders)
	return p
}

// NewWithHTTPClient creates a new OpenAI provider with a custom HTTP client.
// If httpClient is nil, http.DefaultClient is used.
func NewWithHTTPClient(apiKey string, httpClient *http.Client, hooks llmclient.Hooks) *Provider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	p := &Provider{apiKey: apiKey}
	cfg := llmclient.DefaultConfig("openai", defaultBaseURL)
	cfg.Hooks = hooks
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

	// Forward request ID if present in context using OpenAI's X-Client-Request-Id header.
	// OpenAI requires ASCII-only characters and max 512 bytes, otherwise returns 400.
	if requestID := core.GetRequestID(req.Context()); requestID != "" && isValidClientRequestID(requestID) {
		req.Header.Set("X-Client-Request-Id", requestID)
	}
}

// isValidClientRequestID checks if the request ID is valid for OpenAI's X-Client-Request-Id header.
// OpenAI requires: ASCII characters only, max 512 characters.
func isValidClientRequestID(id string) bool {
	if len(id) > 512 {
		return false
	}
	for i := 0; i < len(id); i++ {
		if id[i] > 127 {
			return false
		}
	}
	return true
}

// isOSeriesModel reports whether the model is an OpenAI o-series model
// (o1, o3, o4) that requires max_completion_tokens instead of max_tokens
// and does not support the temperature parameter.
func isOSeriesModel(model string) bool {
	m := strings.ToLower(model)
	// Match o1, o3, o4 families (e.g. o3-mini, o4-mini, o3, o1-preview).
	// Non-reasoning models like gpt-4o start with "gpt-", not "o".
	return len(m) >= 2 && m[0] == 'o' && m[1] >= '0' && m[1] <= '9'
}

// oSeriesChatRequest is the JSON body sent to OpenAI for o-series models.
// It uses max_completion_tokens (required) instead of max_tokens (rejected).
type oSeriesChatRequest struct {
	Model              string              `json:"model"`
	Messages           []core.Message      `json:"messages"`
	Stream             bool                `json:"stream,omitempty"`
	StreamOptions      *core.StreamOptions `json:"stream_options,omitempty"`
	Reasoning          *core.Reasoning     `json:"reasoning,omitempty"`
	MaxCompletionTokens *int               `json:"max_completion_tokens,omitempty"`
}

// adaptForOSeries converts a ChatRequest into an oSeriesChatRequest,
// mapping max_tokens → max_completion_tokens and dropping temperature.
func adaptForOSeries(req *core.ChatRequest) *oSeriesChatRequest {
	return &oSeriesChatRequest{
		Model:               req.Model,
		Messages:            req.Messages,
		Stream:              req.Stream,
		StreamOptions:       req.StreamOptions,
		Reasoning:           req.Reasoning,
		MaxCompletionTokens: req.MaxTokens,
	}
}

// chatRequestBody returns the appropriate request body for the model.
// Reasoning models get parameter adaptation; others pass through as-is.
func chatRequestBody(req *core.ChatRequest) any {
	if isOSeriesModel(req.Model) {
		return adaptForOSeries(req)
	}
	return req
}

// ChatCompletion sends a chat completion request to OpenAI
func (p *Provider) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	var resp core.ChatResponse
	err := p.client.Do(ctx, llmclient.Request{
		Method:   http.MethodPost,
		Endpoint: "/chat/completions",
		Body:     chatRequestBody(req),
	}, &resp)
	if err != nil {
		return nil, err
	}
	resp.Provider = "openai"
	if resp.Model == "" {
		resp.Model = req.Model
	}
	return &resp, nil
}

// StreamChatCompletion returns a raw response body for streaming (caller must close)
func (p *Provider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	streamReq := req.WithStreaming()
	return p.client.DoStream(ctx, llmclient.Request{
		Method:   http.MethodPost,
		Endpoint: "/chat/completions",
		Body:     chatRequestBody(streamReq),
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
	resp.Provider = "openai"
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

// Embeddings sends an embeddings request to OpenAI
func (p *Provider) Embeddings(ctx context.Context, req *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	var resp core.EmbeddingResponse
	err := p.client.Do(ctx, llmclient.Request{
		Method:   http.MethodPost,
		Endpoint: "/embeddings",
		Body:     req,
	}, &resp)
	if err != nil {
		return nil, err
	}
	resp.Provider = "openai"
	return &resp, nil
}
