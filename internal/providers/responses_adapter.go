package providers

import (
	"context"
	"io"
	"strings"

	"github.com/google/uuid"

	"gomodel/internal/core"
)

// ChatProvider is the minimal interface needed by the shared Responses-to-Chat adapter.
// Any provider that supports ChatCompletion and StreamChatCompletion can use the
// ResponsesViaChat and StreamResponsesViaChat helpers to implement the Responses API.
type ChatProvider interface {
	ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error)
	StreamChatCompletion(ctx context.Context, req *core.ChatRequest) (io.ReadCloser, error)
}

// ConvertResponsesRequestToChat converts a ResponsesRequest to a ChatRequest.
func ConvertResponsesRequestToChat(req *core.ResponsesRequest) *core.ChatRequest {
	chatReq := &core.ChatRequest{
		Model:       req.Model,
		Provider:    req.Provider,
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
				content := ExtractContentFromInput(msgMap["content"])
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

// ExtractContentFromInput extracts text content from responses input.
func ExtractContentFromInput(content interface{}) string {
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

// ConvertChatResponseToResponses converts a ChatResponse to a ResponsesResponse.
func ConvertChatResponseToResponses(resp *core.ChatResponse) *core.ResponsesResponse {
	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	return &core.ResponsesResponse{
		ID:        resp.ID,
		Object:    "response",
		CreatedAt: resp.Created,
		Model:     resp.Model,
		Provider:  resp.Provider,
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

// ResponsesViaChat implements the Responses API by converting to/from Chat format.
func ResponsesViaChat(ctx context.Context, p ChatProvider, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	chatReq := ConvertResponsesRequestToChat(req)

	chatResp, err := p.ChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	return ConvertChatResponseToResponses(chatResp), nil
}

// StreamResponsesViaChat implements streaming Responses API by converting to/from Chat format.
func StreamResponsesViaChat(ctx context.Context, p ChatProvider, req *core.ResponsesRequest, providerName string) (io.ReadCloser, error) {
	chatReq := ConvertResponsesRequestToChat(req)

	stream, err := p.StreamChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	return NewOpenAIResponsesStreamConverter(stream, req.Model, providerName), nil
}
