// Package openai provides OpenAI API integration for the LLM gateway.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"gomodel/internal/core"
	"gomodel/internal/pkg/httpclient"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
)

// Provider implements the core.Provider interface for OpenAI
type Provider struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

// New creates a new OpenAI provider
func New(apiKey string) *Provider {
	return &Provider{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		httpClient: httpclient.NewDefaultHTTPClient(),
	}
}

// NewWithHTTPClient creates a new OpenAI provider with a custom HTTP client
func NewWithHTTPClient(apiKey string, client *http.Client) *Provider {
	return &Provider{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		httpClient: client,
	}
}

// Supports returns true if this provider can handle the given model
func (p *Provider) Supports(model string) bool {
	return strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o1")
}

// ChatCompletion sends a chat completion request to OpenAI
func (p *Provider) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, core.NewInvalidRequestError("failed to marshal request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, core.NewInvalidRequestError("failed to create request", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, core.NewProviderError("openai", http.StatusBadGateway, "failed to send request: "+err.Error(), err)
	}
	defer func() {
		_ = resp.Body.Close() //nolint:errcheck
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, core.NewProviderError("openai", http.StatusBadGateway, "failed to read response: "+err.Error(), err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, core.ParseProviderError("openai", resp.StatusCode, respBody, nil)
	}

	var chatResp core.ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, core.NewProviderError("openai", http.StatusBadGateway, "failed to unmarshal response: "+err.Error(), err)
	}

	return &chatResp, nil
}

// StreamChatCompletion returns a raw response body for streaming (caller must close)
func (p *Provider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, core.NewInvalidRequestError("failed to marshal request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, core.NewInvalidRequestError("failed to create request", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, core.NewProviderError("openai", http.StatusBadGateway, "failed to send request: "+err.Error(), err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			respBody = []byte("failed to read error response")
		}
		_ = resp.Body.Close() //nolint:errcheck
		return nil, core.ParseProviderError("openai", resp.StatusCode, respBody, nil)
	}

	return resp.Body, nil
}

// ListModels retrieves the list of available models from OpenAI
func (p *Provider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/models", nil)
	if err != nil {
		return nil, core.NewInvalidRequestError("failed to create request", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, core.NewProviderError("openai", http.StatusBadGateway, "failed to send request: "+err.Error(), err)
	}
	defer func() {
		_ = resp.Body.Close() //nolint:errcheck
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, core.NewProviderError("openai", http.StatusBadGateway, "failed to read response: "+err.Error(), err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, core.ParseProviderError("openai", resp.StatusCode, respBody, nil)
	}

	var modelsResp core.ModelsResponse
	if err := json.Unmarshal(respBody, &modelsResp); err != nil {
		return nil, core.NewProviderError("openai", http.StatusBadGateway, "failed to unmarshal response: "+err.Error(), err)
	}

	return &modelsResp, nil
}
