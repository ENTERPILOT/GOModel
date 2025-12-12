// Package anthropic provides Anthropic API integration for the LLM gateway.
package anthropic

import (
	"bufio"
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
	defaultBaseURL      = "https://api.anthropic.com/v1"
	anthropicAPIVersion = "2023-06-01"
)

func init() {
	// Self-register with the factory
	providers.Register("anthropic", func(cfg config.ProviderConfig) (core.Provider, error) {
		p := New(cfg.APIKey)
		// Override base URL if provided in config
		if cfg.BaseURL != "" {
			p.SetBaseURL(cfg.BaseURL)
		}
		return p, nil
	})
}

// Provider implements the core.Provider interface for Anthropic
type Provider struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

// New creates a new Anthropic provider
func New(apiKey string) *Provider {
	return &Provider{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		httpClient: httpclient.NewDefaultHTTPClient(),
	}
}

// NewWithHTTPClient creates a new Anthropic provider with a custom HTTP client
func NewWithHTTPClient(apiKey string, client *http.Client) *Provider {
	return &Provider{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		httpClient: client,
	}
}

// SetBaseURL allows configuring a custom base URL for the provider
func (p *Provider) SetBaseURL(url string) {
	p.baseURL = url
}

// Supports returns true if this provider can handle the given model
func (p *Provider) Supports(model string) bool {
	return strings.HasPrefix(model, "claude-")
}

// anthropicRequest represents the Anthropic API request format
type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature *float64           `json:"temperature,omitempty"`
	System      string             `json:"system,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

// anthropicMessage represents a message in Anthropic format
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse represents the Anthropic API response format
type anthropicResponse struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Role       string             `json:"role"`
	Content    []anthropicContent `json:"content"`
	Model      string             `json:"model"`
	StopReason string             `json:"stop_reason"`
	Usage      anthropicUsage     `json:"usage"`
}

// anthropicContent represents content in Anthropic response
type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// anthropicUsage represents token usage in Anthropic response
type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// anthropicStreamEvent represents a streaming event from Anthropic
type anthropicStreamEvent struct {
	Type         string             `json:"type"`
	Index        int                `json:"index,omitempty"`
	Delta        *anthropicDelta    `json:"delta,omitempty"`
	ContentBlock *anthropicContent  `json:"content_block,omitempty"`
	Message      *anthropicResponse `json:"message,omitempty"`
	Usage        *anthropicUsage    `json:"usage,omitempty"`
}

// anthropicDelta represents a delta in streaming response
type anthropicDelta struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
}

// convertToAnthropicRequest converts core.ChatRequest to Anthropic format
func convertToAnthropicRequest(req *core.ChatRequest) *anthropicRequest {
	anthropicReq := &anthropicRequest{
		Model:       req.Model,
		Messages:    make([]anthropicMessage, 0, len(req.Messages)),
		MaxTokens:   4096, // Default max tokens
		Temperature: req.Temperature,
		Stream:      req.Stream,
	}

	if req.MaxTokens != nil {
		anthropicReq.MaxTokens = *req.MaxTokens
	}

	// Extract system message if present and convert messages
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			anthropicReq.System = msg.Content
		} else {
			anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	return anthropicReq
}

// convertFromAnthropicResponse converts Anthropic response to core.ChatResponse
func convertFromAnthropicResponse(resp *anthropicResponse) *core.ChatResponse {
	content := ""
	if len(resp.Content) > 0 {
		content = resp.Content[0].Text
	}

	finishReason := resp.StopReason
	if finishReason == "" {
		finishReason = "stop"
	}

	return &core.ChatResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Model:   resp.Model,
		Created: time.Now().Unix(),
		Choices: []core.Choice{
			{
				Index: 0,
				Message: core.Message{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: finishReason,
			},
		},
		Usage: core.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}

// ChatCompletion sends a chat completion request to Anthropic
func (p *Provider) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	anthropicReq := convertToAnthropicRequest(req)

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, core.NewInvalidRequestError("failed to marshal request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, core.NewInvalidRequestError("failed to create request", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, core.NewProviderError("anthropic", http.StatusBadGateway, "failed to send request: "+err.Error(), err)
	}
	defer func() {
		_ = resp.Body.Close() //nolint:errcheck
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, core.NewProviderError("anthropic", http.StatusBadGateway, "failed to read response: "+err.Error(), err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, core.ParseProviderError("anthropic", resp.StatusCode, respBody, nil)
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, core.NewProviderError("anthropic", http.StatusBadGateway, "failed to unmarshal response: "+err.Error(), err)
	}

	return convertFromAnthropicResponse(&anthropicResp), nil
}

// StreamChatCompletion returns a raw response body for streaming (caller must close)
func (p *Provider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	anthropicReq := convertToAnthropicRequest(req)
	anthropicReq.Stream = true

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, core.NewInvalidRequestError("failed to marshal request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, core.NewInvalidRequestError("failed to create request", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, core.NewProviderError("anthropic", http.StatusBadGateway, "failed to send request: "+err.Error(), err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			respBody = []byte("failed to read error response")
		}
		_ = resp.Body.Close() //nolint:errcheck
		return nil, core.ParseProviderError("anthropic", resp.StatusCode, respBody, nil)
	}

	// Return a reader that converts Anthropic SSE format to OpenAI format
	return newStreamConverter(resp.Body, req.Model), nil
}

// streamConverter wraps an Anthropic stream and converts it to OpenAI format
type streamConverter struct {
	reader *bufio.Reader
	body   io.ReadCloser
	model  string
	msgID  string
	buffer []byte
	closed bool
}

func newStreamConverter(body io.ReadCloser, model string) *streamConverter {
	return &streamConverter{
		reader: bufio.NewReader(body),
		body:   body,
		model:  model,
		buffer: make([]byte, 0, 1024),
	}
}

func (sc *streamConverter) Read(p []byte) (n int, err error) {
	if sc.closed {
		return 0, io.EOF
	}

	// If we have buffered data, return it first
	if len(sc.buffer) > 0 {
		n = copy(p, sc.buffer)
		sc.buffer = sc.buffer[n:]
		return n, nil
	}

	// Read the next SSE event from Anthropic
	for {
		line, err := sc.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				// Send final [DONE] message
				doneMsg := "data: [DONE]\n\n"
				n = copy(p, doneMsg)
				if n < len(doneMsg) {
					sc.buffer = append(sc.buffer, []byte(doneMsg)[n:]...)
				}
				sc.closed = true
				_ = sc.body.Close() //nolint:errcheck
				return n, nil
			}
			return 0, err
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// Parse SSE line
		if bytes.HasPrefix(line, []byte("event:")) {
			continue // Skip event type lines
		}

		if bytes.HasPrefix(line, []byte("data:")) {
			data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))

			var event anthropicStreamEvent
			if err := json.Unmarshal(data, &event); err != nil {
				continue
			}

			// Convert Anthropic event to OpenAI format
			openAIChunk := sc.convertEvent(&event)
			if openAIChunk == "" {
				continue
			}

			// Buffer the converted chunk
			sc.buffer = append(sc.buffer, []byte(openAIChunk)...)

			// Return as much as we can
			n = copy(p, sc.buffer)
			sc.buffer = sc.buffer[n:]
			return n, nil
		}
	}
}

func (sc *streamConverter) Close() error {
	sc.closed = true
	return sc.body.Close()
}

func (sc *streamConverter) convertEvent(event *anthropicStreamEvent) string {
	switch event.Type {
	case "message_start":
		if event.Message != nil {
			sc.msgID = event.Message.ID
		}
		return ""

	case "content_block_start":
		return ""

	case "content_block_delta":
		if event.Delta != nil && event.Delta.Text != "" {
			chunk := map[string]interface{}{
				"id":      sc.msgID,
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   sc.model,
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]interface{}{
							"content": event.Delta.Text,
						},
						"finish_reason": nil,
					},
				},
			}
			jsonData, _ := json.Marshal(chunk)
			return fmt.Sprintf("data: %s\n\n", string(jsonData))
		}

	case "content_block_stop":
		return ""

	case "message_delta":
		if event.Delta != nil && event.Delta.StopReason != "" {
			chunk := map[string]interface{}{
				"id":      sc.msgID,
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   sc.model,
				"choices": []map[string]interface{}{
					{
						"index":         0,
						"delta":         map[string]interface{}{},
						"finish_reason": event.Delta.StopReason,
					},
				},
			}
			jsonData, _ := json.Marshal(chunk)
			return fmt.Sprintf("data: %s\n\n", string(jsonData))
		}

	case "message_stop":
		return ""
	}

	return ""
}

// ListModels retrieves the list of available models from Anthropic
func (p *Provider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	// Anthropic doesn't have a models endpoint, so we return a static list
	// of commonly available models
	now := time.Now().Unix()

	models := []core.Model{
		{
			ID:      "claude-3-5-sonnet-20241022",
			Object:  "model",
			OwnedBy: "anthropic",
			Created: now,
		},
		{
			ID:      "claude-3-5-sonnet-20240620",
			Object:  "model",
			OwnedBy: "anthropic",
			Created: now,
		},
		{
			ID:      "claude-3-5-haiku-20241022",
			Object:  "model",
			OwnedBy: "anthropic",
			Created: now,
		},
		{
			ID:      "claude-3-opus-20240229",
			Object:  "model",
			OwnedBy: "anthropic",
			Created: now,
		},
		{
			ID:      "claude-3-sonnet-20240229",
			Object:  "model",
			OwnedBy: "anthropic",
			Created: now,
		},
		{
			ID:      "claude-3-haiku-20240307",
			Object:  "model",
			OwnedBy: "anthropic",
			Created: now,
		},
	}

	return &core.ModelsResponse{
		Object: "list",
		Data:   models,
	}, nil
}

// convertResponsesRequestToAnthropic converts a ResponsesRequest to Anthropic format
func convertResponsesRequestToAnthropic(req *core.ResponsesRequest) *anthropicRequest {
	anthropicReq := &anthropicRequest{
		Model:       req.Model,
		Messages:    make([]anthropicMessage, 0),
		MaxTokens:   4096, // Default max tokens
		Temperature: req.Temperature,
		Stream:      req.Stream,
	}

	if req.MaxOutputTokens != nil {
		anthropicReq.MaxTokens = *req.MaxOutputTokens
	}

	// Set system instruction if provided
	if req.Instructions != "" {
		anthropicReq.System = req.Instructions
	}

	// Convert input to messages
	switch input := req.Input.(type) {
	case string:
		anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{
			Role:    "user",
			Content: input,
		})
	case []interface{}:
		for _, item := range input {
			if msgMap, ok := item.(map[string]interface{}); ok {
				role, _ := msgMap["role"].(string)
				content := extractContentFromResponsesInput(msgMap["content"])
				if role != "" && content != "" {
					anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{
						Role:    role,
						Content: content,
					})
				}
			}
		}
	}

	return anthropicReq
}

// extractContentFromResponsesInput extracts text content from responses input
func extractContentFromResponsesInput(content interface{}) string {
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

// convertAnthropicResponseToResponses converts an Anthropic response to ResponsesResponse
func convertAnthropicResponseToResponses(resp *anthropicResponse, model string) *core.ResponsesResponse {
	content := ""
	if len(resp.Content) > 0 {
		content = resp.Content[0].Text
	}

	return &core.ResponsesResponse{
		ID:        resp.ID,
		Object:    "response",
		CreatedAt: time.Now().Unix(),
		Model:     model,
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
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}

// Responses sends a Responses API request to Anthropic (converted to messages format)
func (p *Provider) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	anthropicReq := convertResponsesRequestToAnthropic(req)

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, core.NewInvalidRequestError("failed to marshal request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, core.NewInvalidRequestError("failed to create request", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, core.NewProviderError("anthropic", http.StatusBadGateway, "failed to send request: "+err.Error(), err)
	}
	defer func() {
		_ = resp.Body.Close() //nolint:errcheck
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, core.NewProviderError("anthropic", http.StatusBadGateway, "failed to read response: "+err.Error(), err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, core.ParseProviderError("anthropic", resp.StatusCode, respBody, nil)
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, core.NewProviderError("anthropic", http.StatusBadGateway, "failed to unmarshal response: "+err.Error(), err)
	}

	return convertAnthropicResponseToResponses(&anthropicResp, req.Model), nil
}

// StreamResponses returns a raw response body for streaming Responses API (caller must close)
func (p *Provider) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	anthropicReq := convertResponsesRequestToAnthropic(req)
	anthropicReq.Stream = true

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, core.NewInvalidRequestError("failed to marshal request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, core.NewInvalidRequestError("failed to create request", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, core.NewProviderError("anthropic", http.StatusBadGateway, "failed to send request: "+err.Error(), err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			respBody = []byte("failed to read error response")
		}
		_ = resp.Body.Close() //nolint:errcheck
		return nil, core.ParseProviderError("anthropic", resp.StatusCode, respBody, nil)
	}

	// Return a reader that converts Anthropic SSE format to Responses API format
	return newResponsesStreamConverter(resp.Body, req.Model), nil
}

// responsesStreamConverter wraps an Anthropic stream and converts it to Responses API format
type responsesStreamConverter struct {
	reader     *bufio.Reader
	body       io.ReadCloser
	model      string
	responseID string
	buffer     []byte
	closed     bool
	sentDone   bool
}

func newResponsesStreamConverter(body io.ReadCloser, model string) *responsesStreamConverter {
	return &responsesStreamConverter{
		reader:     bufio.NewReader(body),
		body:       body,
		model:      model,
		responseID: fmt.Sprintf("resp_%d", time.Now().UnixNano()),
		buffer:     make([]byte, 0, 1024),
	}
}

func (sc *responsesStreamConverter) Read(p []byte) (n int, err error) {
	if sc.closed {
		return 0, io.EOF
	}

	// If we have buffered data, return it first
	if len(sc.buffer) > 0 {
		n = copy(p, sc.buffer)
		sc.buffer = sc.buffer[n:]
		return n, nil
	}

	// Read the next SSE event from Anthropic
	for {
		line, err := sc.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				// Send final done event and [DONE] message
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
					n = copy(p, doneMsg)
					if n < len(doneMsg) {
						sc.buffer = append(sc.buffer, []byte(doneMsg)[n:]...)
					}
					return n, nil
				}
				sc.closed = true
				_ = sc.body.Close() //nolint:errcheck
				return 0, io.EOF
			}
			return 0, err
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// Parse SSE line
		if bytes.HasPrefix(line, []byte("event:")) {
			continue // Skip event type lines
		}

		if bytes.HasPrefix(line, []byte("data:")) {
			data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))

			var event anthropicStreamEvent
			if err := json.Unmarshal(data, &event); err != nil {
				continue
			}

			// Convert Anthropic event to Responses API format
			responsesChunk := sc.convertEvent(&event)
			if responsesChunk == "" {
				continue
			}

			// Buffer the converted chunk
			sc.buffer = append(sc.buffer, []byte(responsesChunk)...)

			// Return as much as we can
			n = copy(p, sc.buffer)
			sc.buffer = sc.buffer[n:]
			return n, nil
		}
	}
}

func (sc *responsesStreamConverter) Close() error {
	sc.closed = true
	return sc.body.Close()
}

func (sc *responsesStreamConverter) convertEvent(event *anthropicStreamEvent) string {
	switch event.Type {
	case "message_start":
		// Send response.created event
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
		return fmt.Sprintf("event: response.created\ndata: %s\n\n", jsonData)

	case "content_block_delta":
		if event.Delta != nil && event.Delta.Text != "" {
			deltaEvent := map[string]interface{}{
				"type":  "response.output_text.delta",
				"delta": event.Delta.Text,
			}
			jsonData, _ := json.Marshal(deltaEvent)
			return fmt.Sprintf("event: response.output_text.delta\ndata: %s\n\n", jsonData)
		}

	case "message_stop":
		// Will be handled in Read() when we get EOF
		return ""
	}

	return ""
}
