// Package anthropic provides Anthropic API integration for the LLM gateway.
package anthropic

import (
	"bufio"
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
	"gomodel/internal/llmclient"
	"gomodel/internal/providers"
)

// Registration provides factory registration for the Anthropic provider.
var Registration = providers.Registration{
	Type: "anthropic",
	New:  New,
}

const (
	defaultBaseURL      = "https://api.anthropic.com/v1"
	anthropicAPIVersion = "2023-06-01"
)

// Provider implements the core.Provider interface for Anthropic
type Provider struct {
	client *llmclient.Client
	apiKey string
}

// New creates a new Anthropic provider.
func New(apiKey string, opts providers.ProviderOptions) core.Provider {
	p := &Provider{apiKey: apiKey}
	cfg := llmclient.Config{
		ProviderName:   "anthropic",
		BaseURL:        defaultBaseURL,
		Retry:          opts.Resilience.Retry,
		Hooks:          opts.Hooks,
		CircuitBreaker: opts.Resilience.CircuitBreaker,
	}
	p.client = llmclient.New(cfg, p.setHeaders)
	return p
}

// NewWithHTTPClient creates a new Anthropic provider with a custom HTTP client.
// If httpClient is nil, http.DefaultClient is used.
func NewWithHTTPClient(apiKey string, httpClient *http.Client, hooks llmclient.Hooks) *Provider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	p := &Provider{apiKey: apiKey}
	cfg := llmclient.DefaultConfig("anthropic", defaultBaseURL)
	cfg.Hooks = hooks
	p.client = llmclient.NewWithHTTPClient(httpClient, cfg, p.setHeaders)
	return p
}

// SetBaseURL allows configuring a custom base URL for the provider
func (p *Provider) SetBaseURL(url string) {
	p.client.SetBaseURL(url)
}

// setHeaders sets the required headers for Anthropic API requests
func (p *Provider) setHeaders(req *http.Request) {
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

	// Forward request ID if present in context
	if requestID := core.GetRequestID(req.Context()); requestID != "" {
		req.Header.Set("X-Request-Id", requestID)
	}
}

// anthropicThinking represents the thinking configuration for Anthropic's extended thinking.
// For 4.6 models: {type: "adaptive"} (budget_tokens omitted).
// For older models: {type: "enabled", budget_tokens: N}.
type anthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// anthropicOutputConfig controls effort level for adaptive thinking on 4.6 models.
type anthropicOutputConfig struct {
	Effort string `json:"effort,omitempty"`
}

// anthropicRequest represents the Anthropic API request format
type anthropicRequest struct {
	Model        string                 `json:"model"`
	Messages     []anthropicMessage     `json:"messages"`
	MaxTokens    int                    `json:"max_tokens"`
	Temperature  *float64               `json:"temperature,omitempty"`
	System       string                 `json:"system,omitempty"`
	Stream       bool                   `json:"stream,omitempty"`
	Thinking     *anthropicThinking     `json:"thinking,omitempty"`
	OutputConfig *anthropicOutputConfig `json:"output_config,omitempty"`
}

var adaptiveThinkingPrefixes = []string{
	"claude-opus-4-6",
	"claude-sonnet-4-6",
}

func isAdaptiveThinkingModel(model string) bool {
	for _, prefix := range adaptiveThinkingPrefixes {
		if model == prefix || strings.HasPrefix(model, prefix+"-") {
			return true
		}
	}
	return false
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
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// anthropicUsage represents token usage in Anthropic response
type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
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

// anthropicModelInfo represents a model in Anthropic's models API response
type anthropicModelInfo struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	CreatedAt   string `json:"created_at"`
	DisplayName string `json:"display_name"`
}

// anthropicModelsResponse represents the Anthropic models API response
type anthropicModelsResponse struct {
	Data    []anthropicModelInfo `json:"data"`
	FirstID string               `json:"first_id"`
	HasMore bool                 `json:"has_more"`
	LastID  string               `json:"last_id"`
}

// normalizeEffort maps effort to gateway-supported values. Anthropic Opus 4.6
// supports "max" for adaptive thinking, but the gateway's public type
// core.Reasoning.Effort only exposes "low", "medium", and "high". "max" is
// therefore intentionally rejected; any unsupported value is downgraded to
// "low" and logged via slog.Warn.
func normalizeEffort(effort string) string {
	switch effort {
	case "low", "medium", "high":
		return effort
	default:
		slog.Warn("invalid reasoning effort, defaulting to 'low'", "effort", effort)
		return "low"
	}
}

// applyReasoning configures thinking and effort on an anthropicRequest.
// Opus 4.6 and Sonnet 4.6 use adaptive thinking with output_config.effort.
// Older models and Haiku 4.6 use manual thinking with budget_tokens.
func applyReasoning(req *anthropicRequest, model, effort string) {
	if isAdaptiveThinkingModel(model) {
		req.Thinking = &anthropicThinking{Type: "adaptive"}
		req.OutputConfig = &anthropicOutputConfig{Effort: normalizeEffort(effort)}
	} else {
		budget := reasoningEffortToBudgetTokens(effort)
		req.Thinking = &anthropicThinking{
			Type:         "enabled",
			BudgetTokens: budget,
		}
		if req.MaxTokens <= budget {
			adjusted := budget + 1024
			slog.Info("MaxTokens adjusted for extended thinking",
				"original", req.MaxTokens, "adjusted", adjusted)
			req.MaxTokens = adjusted
		}
	}

	if req.Temperature != nil {
		if *req.Temperature != 1.0 {
			slog.Warn("temperature overridden to nil; extended thinking requires temperature=1",
				"original_temperature", *req.Temperature)
			req.Temperature = nil
		}
	}
}

func reasoningEffortToBudgetTokens(effort string) int {
	switch normalizeEffort(effort) {
	case "medium":
		return 10000
	case "high":
		return 20000
	default:
		return 5000
	}
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

	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		applyReasoning(anthropicReq, req.Model, req.Reasoning.Effort)
	}

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
	content := extractTextContent(resp.Content)
	toolCalls := extractToolCalls(resp.Content)

	finishReason := resp.StopReason
	if finishReason == "" {
		finishReason = "stop"
	}

	usage := core.Usage{
		PromptTokens:     resp.Usage.InputTokens,
		CompletionTokens: resp.Usage.OutputTokens,
		TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
	}

	rawUsage := buildAnthropicRawUsage(resp.Usage)
	if len(rawUsage) > 0 {
		usage.RawUsage = rawUsage
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
					Role:      "assistant",
					Content:   content,
					ToolCalls: toolCalls,
				},
				FinishReason: finishReason,
			},
		},
		Usage: usage,
	}
}

// ChatCompletion sends a chat completion request to Anthropic
func (p *Provider) ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error) {
	anthropicReq := convertToAnthropicRequest(req)

	var anthropicResp anthropicResponse
	err := p.client.Do(ctx, llmclient.Request{
		Method:   http.MethodPost,
		Endpoint: "/messages",
		Body:     anthropicReq,
	}, &anthropicResp)
	if err != nil {
		return nil, err
	}

	return convertFromAnthropicResponse(&anthropicResp), nil
}

// StreamChatCompletion returns a raw response body for streaming (caller must close)
func (p *Provider) StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error) {
	anthropicReq := convertToAnthropicRequest(req)
	anthropicReq.Stream = true

	stream, err := p.client.DoStream(ctx, llmclient.Request{
		Method:   http.MethodPost,
		Endpoint: "/messages",
		Body:     anthropicReq,
	})
	if err != nil {
		return nil, err
	}

	// Return a reader that converts Anthropic SSE format to OpenAI format
	return newStreamConverter(stream, req.Model), nil
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
				"id":       sc.msgID,
				"object":   "chat.completion.chunk",
				"created":  time.Now().Unix(),
				"model":    sc.model,
				"provider": "anthropic",
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
			jsonData, err := json.Marshal(chunk)
			if err != nil {
				slog.Error("failed to marshal content_block_delta chunk", "error", err, "msg_id", sc.msgID)
				return ""
			}
			return fmt.Sprintf("data: %s\n\n", string(jsonData))
		}

	case "content_block_stop":
		return ""

	case "message_delta":
		// Emit chunk if we have stop_reason or usage data
		if (event.Delta != nil && event.Delta.StopReason != "") || event.Usage != nil {
			var finishReason interface{}
			if event.Delta != nil && event.Delta.StopReason != "" {
				finishReason = event.Delta.StopReason
			}
			chunk := map[string]interface{}{
				"id":       sc.msgID,
				"object":   "chat.completion.chunk",
				"created":  time.Now().Unix(),
				"model":    sc.model,
				"provider": "anthropic",
				"choices": []map[string]interface{}{
					{
						"index":         0,
						"delta":         map[string]interface{}{},
						"finish_reason": finishReason,
					},
				},
			}
			// Include usage data if present (OpenAI format)
			if event.Usage != nil {
				chunk["usage"] = map[string]interface{}{
					"prompt_tokens":     event.Usage.InputTokens,
					"completion_tokens": event.Usage.OutputTokens,
					"total_tokens":      event.Usage.InputTokens + event.Usage.OutputTokens,
				}
			}
			jsonData, err := json.Marshal(chunk)
			if err != nil {
				slog.Error("failed to marshal message_delta chunk", "error", err, "msg_id", sc.msgID)
				return ""
			}
			return fmt.Sprintf("data: %s\n\n", string(jsonData))
		}

	case "message_stop":
		return ""
	}

	return ""
}

// ListModels retrieves the list of available models from Anthropic's /v1/models endpoint
func (p *Provider) ListModels(ctx context.Context) (*core.ModelsResponse, error) {
	var anthropicResp anthropicModelsResponse
	err := p.client.Do(ctx, llmclient.Request{
		Method:   http.MethodGet,
		Endpoint: "/models?limit=1000",
	}, &anthropicResp)
	if err != nil {
		return nil, err
	}

	// Convert to core.Model format
	models := make([]core.Model, 0, len(anthropicResp.Data))
	for _, m := range anthropicResp.Data {
		created := parseCreatedAt(m.CreatedAt)
		models = append(models, core.Model{
			ID:      m.ID,
			Object:  "model",
			OwnedBy: "anthropic",
			Created: created,
		})
	}

	return &core.ModelsResponse{
		Object: "list",
		Data:   models,
	}, nil
}

// parseCreatedAt parses an RFC3339 timestamp string to Unix timestamp
func parseCreatedAt(createdAt string) int64 {
	t, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return time.Now().Unix()
	}
	return t.Unix()
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

	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		applyReasoning(anthropicReq, req.Model, req.Reasoning.Effort)
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

// extractTextContent returns the text from the last "text" content block.
// When extended thinking is enabled, Anthropic returns: [text("\n\n"), thinking(...), text(answer)].
// Taking the last text block ensures we get the actual answer, not the empty preamble.
func extractTextContent(blocks []anthropicContent) string {
	last := ""
	for _, b := range blocks {
		if b.Type == "text" {
			last = b.Text
		}
	}
	return last
}

// extractToolCalls maps Anthropic "tool_use" content blocks to OpenAI-compatible tool calls.
func extractToolCalls(blocks []anthropicContent) []core.ToolCall {
	out := make([]core.ToolCall, 0)
	for _, b := range blocks {
		if b.Type != "tool_use" || b.Name == "" {
			continue
		}

		arguments := "{}"
		if len(b.Input) > 0 {
			arguments = string(b.Input)
		}

		out = append(out, core.ToolCall{
			ID:   b.ID,
			Type: "function",
			Function: core.FunctionCall{
				Name:      b.Name,
				Arguments: arguments,
			},
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// convertAnthropicResponseToResponses converts an Anthropic response to ResponsesResponse
func convertAnthropicResponseToResponses(resp *anthropicResponse, model string) *core.ResponsesResponse {
	content := extractTextContent(resp.Content)

	return &core.ResponsesResponse{
		ID:        resp.ID,
		Object:    "response",
		CreatedAt: time.Now().Unix(),
		Model:     model,
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
		Usage: buildAnthropicResponsesUsage(resp.Usage),
	}
}

// buildAnthropicRawUsage extracts cache fields from anthropicUsage into a RawData map.
func buildAnthropicRawUsage(u anthropicUsage) map[string]any {
	raw := make(map[string]any)
	if u.CacheCreationInputTokens > 0 {
		raw["cache_creation_input_tokens"] = u.CacheCreationInputTokens
	}
	if u.CacheReadInputTokens > 0 {
		raw["cache_read_input_tokens"] = u.CacheReadInputTokens
	}
	if len(raw) == 0 {
		return nil
	}
	return raw
}

// buildAnthropicResponsesUsage creates a ResponsesUsage from anthropicUsage, including RawUsage.
func buildAnthropicResponsesUsage(u anthropicUsage) *core.ResponsesUsage {
	usage := &core.ResponsesUsage{
		InputTokens:  u.InputTokens,
		OutputTokens: u.OutputTokens,
		TotalTokens:  u.InputTokens + u.OutputTokens,
	}
	rawUsage := buildAnthropicRawUsage(u)
	if len(rawUsage) > 0 {
		usage.RawUsage = rawUsage
	}
	return usage
}

// Responses sends a Responses API request to Anthropic (converted to messages format)
func (p *Provider) Responses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	anthropicReq := convertResponsesRequestToAnthropic(req)

	var anthropicResp anthropicResponse
	err := p.client.Do(ctx, llmclient.Request{
		Method:   http.MethodPost,
		Endpoint: "/messages",
		Body:     anthropicReq,
	}, &anthropicResp)
	if err != nil {
		return nil, err
	}

	return convertAnthropicResponseToResponses(&anthropicResp, req.Model), nil
}

// Embeddings returns an error because Anthropic does not natively support embeddings.
// Voyage AI (Anthropic's recommended embedding provider) may be added in the future.
func (p *Provider) Embeddings(_ context.Context, _ *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	return nil, core.NewInvalidRequestError("anthropic does not support embeddings — consider using Voyage AI", nil)
}

// StreamResponses returns a raw response body for streaming Responses API (caller must close)
func (p *Provider) StreamResponses(ctx context.Context, req *core.ResponsesRequest) (io.ReadCloser, error) {
	anthropicReq := convertResponsesRequestToAnthropic(req)
	anthropicReq.Stream = true

	stream, err := p.client.DoStream(ctx, llmclient.Request{
		Method:   http.MethodPost,
		Endpoint: "/messages",
		Body:     anthropicReq,
	})
	if err != nil {
		return nil, err
	}

	// Return a reader that converts Anthropic SSE format to Responses API format
	return newResponsesStreamConverter(stream, req.Model), nil
}

// responsesStreamConverter wraps an Anthropic stream and converts it to Responses API format
type responsesStreamConverter struct {
	reader      *bufio.Reader
	body        io.ReadCloser
	model       string
	responseID  string
	buffer      []byte
	closed      bool
	sentDone    bool
	cachedUsage *anthropicUsage // Stores usage from message_delta for inclusion in response.completed
}

func newResponsesStreamConverter(body io.ReadCloser, model string) *responsesStreamConverter {
	return &responsesStreamConverter{
		reader:     bufio.NewReader(body),
		body:       body,
		model:      model,
		responseID: "resp_" + uuid.New().String(),
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
					responseData := map[string]interface{}{
						"id":         sc.responseID,
						"object":     "response",
						"status":     "completed",
						"model":      sc.model,
						"provider":   "anthropic",
						"created_at": time.Now().Unix(),
					}
					// Include usage data if captured from message_delta
					if sc.cachedUsage != nil {
						responseData["usage"] = map[string]interface{}{
							"input_tokens":  sc.cachedUsage.InputTokens,
							"output_tokens": sc.cachedUsage.OutputTokens,
							"total_tokens":  sc.cachedUsage.InputTokens + sc.cachedUsage.OutputTokens,
						}
					}
					doneEvent := map[string]interface{}{
						"type":     "response.completed",
						"response": responseData,
					}
					jsonData, marshalErr := json.Marshal(doneEvent)
					if marshalErr != nil {
						slog.Error("failed to marshal response.completed event", "error", marshalErr, "response_id", sc.responseID)
						sc.closed = true
						_ = sc.body.Close() //nolint:errcheck
						return 0, io.EOF
					}
					doneMsg := fmt.Sprintf("event: response.completed\ndata: %s\n\ndata: [DONE]\n\n", jsonData)
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
				"provider":   "anthropic",
				"created_at": time.Now().Unix(),
			},
		}
		jsonData, err := json.Marshal(createdEvent)
		if err != nil {
			slog.Error("failed to marshal response.created event", "error", err, "response_id", sc.responseID)
			return ""
		}
		return fmt.Sprintf("event: response.created\ndata: %s\n\n", jsonData)

	case "content_block_delta":
		if event.Delta != nil && event.Delta.Text != "" {
			deltaEvent := map[string]interface{}{
				"type":  "response.output_text.delta",
				"delta": event.Delta.Text,
			}
			jsonData, err := json.Marshal(deltaEvent)
			if err != nil {
				slog.Error("failed to marshal content delta event", "error", err, "response_id", sc.responseID)
				return ""
			}
			return fmt.Sprintf("event: response.output_text.delta\ndata: %s\n\n", jsonData)
		}

	case "message_delta":
		// Capture usage data for inclusion in response.completed
		if event.Usage != nil {
			sc.cachedUsage = event.Usage
		}
		return ""

	case "message_stop":
		// Will be handled in Read() when we get EOF
		return ""
	}

	return ""
}
