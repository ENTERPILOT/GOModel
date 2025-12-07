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

	"gomodel/internal/core"
)

const (
	defaultBaseURL      = "https://api.anthropic.com/v1"
	anthropicAPIVersion = "2023-06-01"
)

// Provider implements the core.Provider interface for Anthropic
type Provider struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

// New creates a new Anthropic provider
func New(apiKey string) *Provider {
	return &Provider{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
			},
		},
	}
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
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() //nolint:errcheck
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Anthropic API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return convertFromAnthropicResponse(&anthropicResp), nil
}

// StreamChatCompletion returns a raw response body for streaming (caller must close)
func (p *Provider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	anthropicReq := convertToAnthropicRequest(req)
	anthropicReq.Stream = true

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			respBody = []byte("failed to read error response")
		}
		_ = resp.Body.Close() //nolint:errcheck
		return nil, fmt.Errorf("Anthropic API error (status %d): %s", resp.StatusCode, string(respBody))
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
