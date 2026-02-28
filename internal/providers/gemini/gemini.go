// Package gemini provides Google Gemini API integration for the LLM gateway.
package gemini

import (
	"context"
	"encoding/json"
	"fmt"
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
	client           *llmclient.Client
	httpClient       *http.Client
	hooks            llmclient.Hooks
	apiKey           string
	modelsURL        string
	modelsClientConf llmclient.Config
}

// New creates a new Gemini provider.
func New(apiKey string, opts providers.ProviderOptions) core.Provider {
	p := &Provider{
		httpClient: nil,
		apiKey:     apiKey,
		hooks:      opts.Hooks,
		modelsURL:  defaultModelsBaseURL,
		modelsClientConf: llmclient.Config{
			ProviderName:   "gemini",
			BaseURL:        defaultModelsBaseURL,
			Retry:          opts.Resilience.Retry,
			Hooks:          opts.Hooks,
			CircuitBreaker: opts.Resilience.CircuitBreaker,
		},
	}
	cfg := llmclient.Config{
		ProviderName:   "gemini",
		BaseURL:        defaultOpenAICompatibleBaseURL,
		Retry:          opts.Resilience.Retry,
		Hooks:          opts.Hooks,
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
		httpClient: httpClient,
		apiKey:     apiKey,
		hooks:      hooks,
		modelsURL:  defaultModelsBaseURL,
	}
	modelsCfg := llmclient.DefaultConfig("gemini", defaultModelsBaseURL)
	modelsCfg.Hooks = hooks
	p.modelsClientConf = modelsCfg
	cfg := llmclient.DefaultConfig("gemini", defaultOpenAICompatibleBaseURL)
	cfg.Hooks = hooks
	p.client = llmclient.NewWithHTTPClient(httpClient, cfg, p.setHeaders)
	return p
}

// SetBaseURL allows configuring a custom base URL for the provider
func (p *Provider) SetBaseURL(url string) {
	p.client.SetBaseURL(url)
}

// SetModelsURL allows configuring a custom models API base URL.
// This is primarily useful for tests and local emulators.
func (p *Provider) SetModelsURL(url string) {
	p.modelsURL = url
	p.modelsClientConf.BaseURL = url
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
	modelsCfg := p.modelsClientConf
	modelsCfg.BaseURL = p.modelsURL
	modelsCfg.Hooks = p.hooks
	headers := func(req *http.Request) {
		// Use header-based API key auth for models requests.
		req.Header.Set("x-goog-api-key", p.apiKey)

		// Preserve request tracing across list-models requests.
		requestID := req.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = core.GetRequestID(req.Context())
		}
		if requestID != "" {
			req.Header.Set("X-Request-Id", requestID)
		}
	}

	var modelsClient *llmclient.Client
	if p.httpClient != nil {
		modelsClient = llmclient.NewWithHTTPClient(p.httpClient, modelsCfg, headers)
	} else {
		modelsClient = llmclient.New(modelsCfg, headers)
	}

	rawResp, err := modelsClient.DoRaw(ctx, llmclient.Request{
		Method:   http.MethodGet,
		Endpoint: "/models",
	})
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()

	// Preferred path: native Gemini models response.
	// If the payload contains an explicit "models" field with an empty array,
	// return an empty list instead of falling through to fallback parsing.
	var nativeProbe struct {
		Models json.RawMessage `json:"models"`
	}
	if err := json.Unmarshal(rawResp.Body, &nativeProbe); err == nil && nativeProbe.Models != nil {
		var geminiResp geminiModelsResponse
		if err := json.Unmarshal(rawResp.Body, &geminiResp); err != nil {
			return nil, core.NewProviderError("gemini", http.StatusBadGateway, "failed to parse native Gemini models response", err)
		}
		if len(geminiResp.Models) == 0 {
			return &core.ModelsResponse{
				Object: "list",
				Data:   []core.Model{},
			}, nil
		}

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

	// Fallback path: OpenAI-compatible models list.
	var openAIResp core.ModelsResponse
	if err := json.Unmarshal(rawResp.Body, &openAIResp); err == nil && openAIResp.Object == "list" {
		models := make([]core.Model, 0, len(openAIResp.Data))
		for _, m := range openAIResp.Data {
			modelID := strings.TrimPrefix(m.ID, "models/")
			isOpenAICompatModel := strings.HasPrefix(modelID, "gemini-") || strings.HasPrefix(modelID, "text-embedding-")
			if !isOpenAICompatModel {
				continue
			}
			models = append(models, core.Model{
				ID:      modelID,
				Object:  "model",
				OwnedBy: "google",
				Created: now,
			})
		}
		return &core.ModelsResponse{
			Object: "list",
			Data:   models,
		}, nil
	}

	responsePreview := string(rawResp.Body)
	if len(responsePreview) > 512 {
		responsePreview = responsePreview[:512] + "...(truncated)"
	}
	return nil, core.NewProviderError("gemini", http.StatusBadGateway, "unexpected Gemini models response format", fmt.Errorf("models response body: %s", responsePreview))
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
