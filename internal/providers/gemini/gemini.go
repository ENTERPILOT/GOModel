// Package gemini provides Google Gemini API integration for the LLM gateway.
package gemini

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"gomodel/internal/core"
	"gomodel/internal/llmclient"
	"gomodel/internal/providers"
)

const (
	// Gemini provides an OpenAI-compatible endpoint
	defaultOpenAICompatibleBaseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
	// Native Gemini API endpoint for models listing
	defaultModelsBaseURL = "https://generativelanguage.googleapis.com/v1beta"
)

func init() {
	// Self-register with the factory
	providers.RegisterProvider("gemini", New)
}

// Provider implements the core.Provider interface for Google Gemini
type Provider struct {
	client    *llmclient.Client
	apiKey    string
	modelsURL string
}

// New creates a new Gemini provider
func New(apiKey string) *Provider {
	p := &Provider{
		apiKey:    apiKey,
		modelsURL: defaultModelsBaseURL,
	}
	cfg := llmclient.DefaultConfig("gemini", defaultOpenAICompatibleBaseURL)
	// Apply global hooks if available
	cfg.Hooks = providers.GetGlobalHooks()
	p.client = llmclient.New(cfg, p.setHeaders)
	return p
}

// NewWithHTTPClient creates a new Gemini provider with a custom HTTP client
func NewWithHTTPClient(apiKey string, httpClient *http.Client) *Provider {
	p := &Provider{
		apiKey:    apiKey,
		modelsURL: defaultModelsBaseURL,
	}
	cfg := llmclient.DefaultConfig("gemini", defaultOpenAICompatibleBaseURL)
	// Apply global hooks if available
	cfg.Hooks = providers.GetGlobalHooks()
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
	// Apply global hooks if available
	modelsCfg.Hooks = providers.GetGlobalHooks()
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

		if supportsGenerate && strings.HasPrefix(modelID, "gemini-") {
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

// convertResponsesRequestToChat converts a ResponsesRequest to ChatRequest for Gemini
func convertResponsesRequestToChat(req *core.ResponsesRequest) *core.ChatRequest {
	chatReq := &core.ChatRequest{
		Model:       req.Model,
		Messages:    make([]core.Message, 0),
		Temperature: req.Temperature,
		Stream:      req.Stream,
	}

	if req.MaxOutputTokens != nil {
		chatReq.MaxTokens = req.MaxOutputTokens
	}

	// Add system instruction if provided
	if req.Instructions != "" {
		chatReq.Messages = append(chatReq.Messages, core.Message{
			Role:    "system",
			Content: req.Instructions,
		})
	}

	// Convert input to messages
	switch input := req.Input.(type) {
	case string:
		chatReq.Messages = append(chatReq.Messages, core.Message{
			Role:    "user",
			Content: input,
		})
	case []interface{}:
		for _, item := range input {
			if msgMap, ok := item.(map[string]interface{}); ok {
				role, _ := msgMap["role"].(string)
				content := extractContentFromInput(msgMap["content"])
				if role != "" && content != "" {
					chatReq.Messages = append(chatReq.Messages, core.Message{
						Role:    role,
						Content: content,
					})
				}
			}
		}
	}

	return chatReq
}

// extractContentFromInput extracts text content from responses input
func extractContentFromInput(content interface{}) string {
	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		// Array of content parts - extract text
		var texts []string
		for _, part := range c {
			if partMap, ok := part.(map[string]interface{}); ok {
				if text, ok := partMap["text"].(string); ok {
					texts = append(texts, text)
				}
			}
		}
		return strings.Join(texts, " ")
	}
	return ""
}

// convertChatResponseToResponses converts a ChatResponse to ResponsesResponse
func convertChatResponseToResponses(resp *core.ChatResponse) *core.ResponsesResponse {
	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	return &core.ResponsesResponse{
		ID:        resp.ID,
		Object:    "response",
		CreatedAt: resp.Created,
		Model:     resp.Model,
		Status:    "completed",
		Output: []core.ResponsesOutputItem{
			{
				ID:     "msg_" + uuid.New().String(),
				Type:   "message",
				Role:   "assistant",
				Status: "completed",
				Content: []core.ResponsesContentItem{
					{
						Type:        "output_text",
						Text:        content,
						Annotations: []string{},
					},
				},
			},
		},
		Usage: &core.ResponsesUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
	}
}

// Responses sends a Responses API request to Gemini (converted to chat format)
func (p *Provider) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	// Convert ResponsesRequest to ChatRequest
	chatReq := convertResponsesRequestToChat(req)

	// Use the existing ChatCompletion method
	chatResp, err := p.ChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	return convertChatResponseToResponses(chatResp), nil
}

// StreamResponses returns a raw response body for streaming Responses API (caller must close)
func (p *Provider) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	// Convert ResponsesRequest to ChatRequest
	chatReq := convertResponsesRequestToChat(req)

	// Get the streaming response from chat completions
	stream, err := p.StreamChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	// Wrap the stream to convert chat completion format to Responses API format
	return providers.NewOpenAIResponsesStreamConverter(stream, req.Model), nil
}
