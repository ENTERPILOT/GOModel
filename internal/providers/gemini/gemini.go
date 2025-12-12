// Package gemini provides Google Gemini API integration for the LLM gateway.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gomodel/config"
	"gomodel/internal/core"
	"gomodel/internal/pkg/httpclient"
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
	providers.Register("gemini", func(cfg config.ProviderConfig) (core.Provider, error) {
		p := New(cfg.APIKey)
		// Override base URL if provided in config
		if cfg.BaseURL != "" {
			p.SetBaseURL(cfg.BaseURL)
		}
		return p, nil
	})
}

// Provider implements the core.Provider interface for Google Gemini
type Provider struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
	modelsURL  string
}

// New creates a new Gemini provider
func New(apiKey string) *Provider {
	return &Provider{
		apiKey:     apiKey,
		baseURL:    defaultOpenAICompatibleBaseURL,
		modelsURL:  defaultModelsBaseURL,
		httpClient: httpclient.NewDefaultHTTPClient(),
	}
}

// NewWithHTTPClient creates a new Gemini provider with a custom HTTP client
func NewWithHTTPClient(apiKey string, client *http.Client) *Provider {
	return &Provider{
		apiKey:     apiKey,
		baseURL:    defaultOpenAICompatibleBaseURL,
		modelsURL:  defaultModelsBaseURL,
		httpClient: client,
	}
}

// SetBaseURL allows configuring a custom base URL for the provider
func (p *Provider) SetBaseURL(url string) {
	p.baseURL = url
}

// Supports returns true if this provider can handle the given model
func (p *Provider) Supports(model string) bool {
	return strings.HasPrefix(model, "gemini-")
}

// ChatCompletion sends a chat completion request to Gemini
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
		return nil, core.NewProviderError("gemini", http.StatusBadGateway, "failed to send request: "+err.Error(), err)
	}
	defer func() {
		_ = resp.Body.Close() //nolint:errcheck
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, core.NewProviderError("gemini", http.StatusBadGateway, "failed to read response: "+err.Error(), err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, core.ParseProviderError("gemini", resp.StatusCode, respBody, nil)
	}

	var chatResp core.ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, core.NewProviderError("gemini", http.StatusBadGateway, "failed to unmarshal response: "+err.Error(), err)
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
		return nil, core.NewProviderError("gemini", http.StatusBadGateway, "failed to send request: "+err.Error(), err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			respBody = []byte("failed to read error response")
		}
		_ = resp.Body.Close() //nolint:errcheck
		return nil, core.ParseProviderError("gemini", resp.StatusCode, respBody, nil)
	}

	// Gemini's OpenAI-compatible endpoint returns OpenAI-format SSE, so we can pass it through directly
	return resp.Body, nil
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
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.modelsURL+"/models", nil)
	if err != nil {
		return nil, core.NewInvalidRequestError("failed to create request", err)
	}

	// Add API key as query parameter.
	// NOTE: Passing the API key in the URL query parameter is required by Google's native Gemini API for the models endpoint.
	// This may be a security concern, as the API key can be logged in server access logs, proxy logs, and browser history.
	// See: https://cloud.google.com/vertex-ai/docs/generative-ai/model-parameters#api-key
	q := httpReq.URL.Query()
	q.Add("key", p.apiKey)
	httpReq.URL.RawQuery = q.Encode()

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, core.NewProviderError("gemini", http.StatusBadGateway, "failed to send request: "+err.Error(), err)
	}
	defer func() {
		_ = resp.Body.Close() //nolint:errcheck
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, core.NewProviderError("gemini", http.StatusBadGateway, "failed to read response: "+err.Error(), err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, core.ParseProviderError("gemini", resp.StatusCode, respBody, nil)
	}

	var geminiResp geminiModelsResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, core.NewProviderError("gemini", http.StatusBadGateway, "failed to unmarshal response: "+err.Error(), err)
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
				ID:     fmt.Sprintf("msg_%d", time.Now().UnixNano()),
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
	chatReq.Stream = true

	// Get the streaming response from chat completions
	stream, err := p.StreamChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	// Wrap the stream to convert chat completion format to Responses API format
	return newGeminiResponsesStreamConverter(stream, req.Model), nil
}

// geminiResponsesStreamConverter wraps a chat completion stream and converts it to Responses API format
type geminiResponsesStreamConverter struct {
	reader     io.ReadCloser
	model      string
	responseID string
	buffer     []byte
	lineBuffer []byte
	closed     bool
	sentCreate bool
	sentDone   bool
}

func newGeminiResponsesStreamConverter(reader io.ReadCloser, model string) *geminiResponsesStreamConverter {
	return &geminiResponsesStreamConverter{
		reader:     reader,
		model:      model,
		responseID: "resp_" + time.Now().Format("20060102150405"),
		buffer:     make([]byte, 0, 4096),
		lineBuffer: make([]byte, 0, 1024),
	}
}

func (sc *geminiResponsesStreamConverter) Read(p []byte) (n int, err error) {
	if sc.closed {
		return 0, io.EOF
	}

	// If we have buffered data, return it first
	if len(sc.buffer) > 0 {
		n = copy(p, sc.buffer)
		sc.buffer = sc.buffer[n:]
		return n, nil
	}

	// Send response.created event first
	if !sc.sentCreate {
		sc.sentCreate = true
		createdEvent := map[string]interface{}{
			"type": "response.created",
			"response": map[string]interface{}{
				"id":         sc.responseID,
				"object":     "response",
				"status":     "in_progress",
				"model":      sc.model,
				"created_at": time.Now().Unix(),
			},
		}
		jsonData, _ := json.Marshal(createdEvent)
		created := fmt.Sprintf("event: response.created\ndata: %s\n\n", jsonData)
		sc.buffer = append(sc.buffer, []byte(created)...)
		n = copy(p, sc.buffer)
		sc.buffer = sc.buffer[n:]
		return n, nil
	}

	// Read from the underlying stream
	tempBuf := make([]byte, 1024)
	nr, readErr := sc.reader.Read(tempBuf)
	if nr > 0 {
		sc.lineBuffer = append(sc.lineBuffer, tempBuf[:nr]...)

		// Process complete lines
		for {
			idx := bytes.Index(sc.lineBuffer, []byte("\n"))
			if idx == -1 {
				break
			}

			line := sc.lineBuffer[:idx]
			sc.lineBuffer = sc.lineBuffer[idx+1:]

			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}

			if bytes.HasPrefix(line, []byte("data: ")) {
				data := bytes.TrimPrefix(line, []byte("data: "))
				if bytes.Equal(data, []byte("[DONE]")) {
					// Send done event
					if !sc.sentDone {
						sc.sentDone = true
						doneEvent := map[string]interface{}{
							"type": "response.done",
							"response": map[string]interface{}{
								"id":         sc.responseID,
								"object":     "response",
								"status":     "completed",
								"model":      sc.model,
								"created_at": time.Now().Unix(),
							},
						}
						jsonData, _ := json.Marshal(doneEvent)
						doneMsg := fmt.Sprintf("event: response.done\ndata: %s\n\ndata: [DONE]\n\n", jsonData)
						sc.buffer = append(sc.buffer, []byte(doneMsg)...)
					}
					continue
				}

				// Parse the chat completion chunk
				var chunk map[string]interface{}
				if err := json.Unmarshal(data, &chunk); err != nil {
					continue
				}

				// Extract content delta
				if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
					if choice, ok := choices[0].(map[string]interface{}); ok {
						if delta, ok := choice["delta"].(map[string]interface{}); ok {
							if content, ok := delta["content"].(string); ok && content != "" {
								deltaEvent := map[string]interface{}{
									"type":  "response.output_text.delta",
									"delta": content,
								}
								jsonData, _ := json.Marshal(deltaEvent)
								sc.buffer = append(sc.buffer, []byte(fmt.Sprintf("event: response.output_text.delta\ndata: %s\n\n", jsonData))...)
							}
						}
					}
				}
			}
		}
	}

	if readErr != nil {
		if readErr == io.EOF {
			// Send final done event if we haven't already
			if !sc.sentDone {
				sc.sentDone = true
				doneEvent := map[string]interface{}{
					"type": "response.done",
					"response": map[string]interface{}{
						"id":         sc.responseID,
						"object":     "response",
						"status":     "completed",
						"model":      sc.model,
						"created_at": time.Now().Unix(),
					},
				}
				jsonData, _ := json.Marshal(doneEvent)
				doneMsg := fmt.Sprintf("event: response.done\ndata: %s\n\ndata: [DONE]\n\n", jsonData)
				sc.buffer = append(sc.buffer, []byte(doneMsg)...)
			}

			if len(sc.buffer) > 0 {
				n = copy(p, sc.buffer)
				sc.buffer = sc.buffer[n:]
				return n, nil
			}

			sc.closed = true
			_ = sc.reader.Close()
			return 0, io.EOF
		}
		return 0, readErr
	}

	if len(sc.buffer) > 0 {
		n = copy(p, sc.buffer)
		sc.buffer = sc.buffer[n:]
		return n, nil
	}

	// No data yet, try again
	return 0, nil
}

func (sc *geminiResponsesStreamConverter) Close() error {
	sc.closed = true
	return sc.reader.Close()
}
