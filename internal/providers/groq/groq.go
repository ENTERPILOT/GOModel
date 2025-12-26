// Package groq provides Groq API integration for the LLM gateway.
package groq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"gomodel/internal/core"
	"gomodel/internal/pkg/llmclient"
	"gomodel/internal/providers"
)

const (
	defaultBaseURL = "https://api.groq.com/openai/v1"
)

func init() {
	// Self-register with the factory
	providers.RegisterProvider("groq", New)
}

// Provider implements the core.Provider interface for Groq
type Provider struct {
	client *llmclient.Client
	apiKey string
}

// New creates a new Groq provider
func New(apiKey string) *Provider {
	p := &Provider{apiKey: apiKey}
	cfg := llmclient.DefaultConfig("groq", defaultBaseURL)
	// Apply global hooks if available
	cfg.Hooks = providers.GetGlobalHooks()
	p.client = llmclient.New(cfg, p.setHeaders)
	return p
}

// NewWithHTTPClient creates a new Groq provider with a custom HTTP client
func NewWithHTTPClient(apiKey string, httpClient *http.Client) *Provider {
	p := &Provider{apiKey: apiKey}
	cfg := llmclient.DefaultConfig("groq", defaultBaseURL)
	// Apply global hooks if available
	cfg.Hooks = providers.GetGlobalHooks()
	p.client = llmclient.NewWithHTTPClient(httpClient, cfg, p.setHeaders)
	return p
}

// SetBaseURL allows configuring a custom base URL for the provider
func (p *Provider) SetBaseURL(url string) {
	p.client.SetBaseURL(url)
}

// setHeaders sets the required headers for Groq API requests
func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
}

// ChatCompletion sends a chat completion request to Groq
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

// ListModels retrieves the list of available models from Groq
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

// convertResponsesRequestToChat converts a ResponsesRequest to ChatRequest for Groq
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

// Responses sends a Responses API request to Groq (converted to chat format)
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
	return newGroqResponsesStreamConverter(stream, req.Model), nil
}

// groqResponsesStreamConverter wraps a chat completion stream and converts it to Responses API format
type groqResponsesStreamConverter struct {
	reader     io.ReadCloser
	model      string
	responseID string
	buffer     []byte
	lineBuffer []byte
	closed     bool
	sentCreate bool
	sentDone   bool
}

func newGroqResponsesStreamConverter(reader io.ReadCloser, model string) *groqResponsesStreamConverter {
	return &groqResponsesStreamConverter{
		reader:     reader,
		model:      model,
		responseID: "resp_" + uuid.New().String(),
		buffer:     make([]byte, 0, 4096),
		lineBuffer: make([]byte, 0, 1024),
	}
}

func (sc *groqResponsesStreamConverter) Read(p []byte) (n int, err error) {
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
		jsonData, err := json.Marshal(createdEvent)
		if err != nil {
			slog.Error("failed to marshal response.created event", "error", err, "response_id", sc.responseID)
			return 0, nil
		}
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
						jsonData, err := json.Marshal(doneEvent)
						if err != nil {
							slog.Error("failed to marshal response.done event", "error", err, "response_id", sc.responseID)
							continue
						}
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
								jsonData, err := json.Marshal(deltaEvent)
								if err != nil {
									slog.Error("failed to marshal content delta event", "error", err, "response_id", sc.responseID)
									continue
								}
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
				jsonData, err := json.Marshal(doneEvent)
				if err != nil {
					slog.Error("failed to marshal final response.done event", "error", err, "response_id", sc.responseID)
				} else {
					doneMsg := fmt.Sprintf("event: response.done\ndata: %s\n\ndata: [DONE]\n\n", jsonData)
					sc.buffer = append(sc.buffer, []byte(doneMsg)...)
				}
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

func (sc *groqResponsesStreamConverter) Close() error {
	sc.closed = true
	return sc.reader.Close()
}
