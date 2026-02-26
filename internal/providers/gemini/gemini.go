// Package gemini provides Google Gemini API integration for the LLM gateway.
package gemini

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"gomodel/internal/core"
	"gomodel/internal/llmclient"
	"gomodel/internal/providers"
)

// Registration provides factory registration for the Gemini provider.
var Registration = providers.Registration{
	Type: "gemini",
	New:  New,
}

const (
	// Gemini provides an OpenAI-compatible endpoint
	defaultOpenAICompatibleBaseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
	// Native Gemini API endpoint for models listing
	defaultModelsBaseURL = "https://generativelanguage.googleapis.com/v1beta"
)

// Provider implements the core.Provider interface for Google Gemini
type Provider struct {
	client    *llmclient.Client
	hooks     llmclient.Hooks
	apiKey    string
	modelsURL string
}

// New creates a new Gemini provider.
func New(apiKey string, opts providers.ProviderOptions) core.Provider {
	p := &Provider{
		apiKey:    apiKey,
		hooks:     opts.Hooks,
		modelsURL: defaultModelsBaseURL,
	}
	cfg := llmclient.Config{
		ProviderName: "gemini",
		BaseURL:      defaultOpenAICompatibleBaseURL,
		Retry:        opts.Resilience.Retry,
		Hooks:        opts.Hooks,
		CircuitBreaker: opts.Resilience.CircuitBreaker,
	}
	p.client = llmclient.New(cfg, p.setHeaders)
	return p
}

// NewWithHTTPClient creates a new Gemini provider with a custom HTTP client.
// If httpClient is nil, http.DefaultClient is used.
func NewWithHTTPClient(apiKey string, httpClient *http.Client, hooks llmclient.Hooks) *Provider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	p := &Provider{
		apiKey:    apiKey,
		hooks:     hooks,
		modelsURL: defaultModelsBaseURL,
	}
	cfg := llmclient.DefaultConfig("gemini", defaultOpenAICompatibleBaseURL)
	cfg.Hooks = hooks
	p.client = llmclient.NewWithHTTPClient(httpClient, cfg, p.setHeaders)
	return p
}

// SetBaseURL allows configuring a custom base URL for the provider
func (p *Provider) SetBaseURL(url string) {
	p.client.SetBaseURL(url)
}

// setHeaders sets the required headers for Gemini API requests
func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	// Forward request ID if present in context for request tracing
	if requestID := core.GetRequestID(req.Context()); requestID != "" {
		req.Header.Set("X-Request-Id", requestID)
	}
}

// ChatCompletion sends a chat completion request to Gemini
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
	if resp.Model == "" {
		resp.Model = req.Model
	}
	return &resp, nil
}

// StreamChatCompletion returns a raw response body for streaming (caller must close)
func (p *Provider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	stream, err := p.client.DoStream(ctx, llmclient.Request{
		Method:   http.MethodPost,
		Endpoint: "/chat/completions",
		Body:     req.WithStreaming(),
	})
	if err != nil {
		return nil, err
	}

	// Gemini's OpenAI-compatible endpoint returns OpenAI-format SSE, so we can pass it through directly
	return stream, nil
}

// geminiModel represents a model in Gemini's native API response
type geminiModel struct {
	Name             string   `json:"name"`
	DisplayName      string   `json:"displayName"`
	Description      string   `json:"description"`
	SupportedMethods []string `json:"supportedGenerationMethods"`
	InputTokenLimit  int      `json:"inputTokenLimit"`
	OutputTokenLimit int      `json:"outputTokenLimit"`
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"topP,omitempty"`
	TopK             *int     `json:"topK,omitempty"`
}

// geminiModelsResponse represents the native Gemini models list response
type geminiModelsResponse struct {
	Models []geminiModel `json:"models"`
}

// ListModels retrieves the list of available models from Gemini
func (p *Provider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	// Use the native Gemini API to list models
	// We need to create a separate client for the models endpoint since it uses a different URL
	modelsCfg := llmclient.DefaultConfig("gemini", p.modelsURL)
	modelsCfg.Hooks = p.hooks
	modelsClient := llmclient.New(
		modelsCfg,
		func(req *http.Request) {
			// Add API key as query parameter.
			// NOTE: Passing the API key in the URL query parameter is required by Google's native Gemini API for the models endpoint.
			// This may be a security concern, as the API key can be logged in server access logs, proxy logs, and browser history.
			// See: https://cloud.google.com/vertex-ai/docs/generative-ai/model-parameters#api-key
			q := req.URL.Query()
			q.Add("key", p.apiKey)
			req.URL.RawQuery = q.Encode()
		},
	)

	var geminiResp geminiModelsResponse
	err := modelsClient.Do(ctx, llmclient.Request{
		Method:   http.MethodGet,
		Endpoint: "/models",
	}, &geminiResp)
	if err != nil {
		return nil, err
	}

	// Convert Gemini models to core.Model format
	now := time.Now().Unix()
	models := make([]core.Model, 0, len(geminiResp.Models))

	for _, gm := range geminiResp.Models {
		// Extract model ID from name (format: "models/gemini-...")
		modelID := strings.TrimPrefix(gm.Name, "models/")

		// Only include models that support generateContent (chat/completion)
		supportsGenerate := false
		for _, method := range gm.SupportedMethods {
			if method == "generateContent" || method == "streamGenerateContent" {
				supportsGenerate = true
				break
			}
		}

		supportsEmbed := false
		for _, method := range gm.SupportedMethods {
			if method == "embedContent" {
				supportsEmbed = true
				break
			}
		}

		isOpenAICompatModel := strings.HasPrefix(modelID, "gemini-") || strings.HasPrefix(modelID, "text-embedding-")
		if (supportsGenerate || supportsEmbed) && isOpenAICompatModel {
			models = append(models, core.Model{
			models = append(models, core.Model{
				ID:      modelID,
				Object:  "model",
				OwnedBy: "google",
				Created: now,
			})
		}
	}

	return &core.ModelsResponse{
		Object: "list",
		Data:   models,
	}, nil
}

// Responses sends a Responses API request to Gemini (converted to chat format)
func (p *Provider) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	return providers.ResponsesViaChat(ctx, p, req)
}

// Embeddings sends an embeddings request to Gemini via its OpenAI-compatible endpoint
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
	if resp.Model == "" {
		resp.Model = req.Model
	}
	return &resp, nil
}

// StreamResponses returns a raw response body for streaming Responses API (caller must close)
func (p *Provider) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	return providers.StreamResponsesViaChat(ctx, p, req, "gemini")
}
