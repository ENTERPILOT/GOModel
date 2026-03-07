package providers

import (
	"context"
	"fmt"
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
// It also validates the supported Responses input shapes and returns an error
// when the request cannot be converted safely.
func ConvertResponsesRequestToChat(req *core.ResponsesRequest) (*core.ChatRequest, error) {
	chatReq := &core.ChatRequest{
		Model:         req.Model,
		Provider:      req.Provider,
		Messages:      make([]core.Message, 0),
		Temperature:   req.Temperature,
		Stream:        req.Stream,
		StreamOptions: req.StreamOptions,
		Reasoning:     req.Reasoning,
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
		for i, item := range input {
			msg, err := convertResponsesInputItemToChatMessage(item, i)
			if err != nil {
				return nil, err
			}
			chatReq.Messages = append(chatReq.Messages, msg)
		}
	case []core.ResponsesInputItem:
		for i, item := range input {
			msg, err := convertResponsesInputItemToChatMessage(item, i)
			if err != nil {
				return nil, err
			}
			chatReq.Messages = append(chatReq.Messages, msg)
		}
	case core.ResponsesInputItem:
		msg, err := convertResponsesInputItemToChatMessage(input, 0)
		if err != nil {
			return nil, err
		}
		chatReq.Messages = append(chatReq.Messages, msg)
	case nil:
		return nil, core.NewInvalidRequestError("invalid responses input: unsupported type", nil)
	default:
		return nil, core.NewInvalidRequestError("invalid responses input: unsupported type", nil)
	}

	return chatReq, nil
}

func convertResponsesInputItemToChatMessage(item any, index int) (core.Message, error) {
	switch v := item.(type) {
	case map[string]interface{}:
		role, _ := v["role"].(string)
		role = strings.TrimSpace(role)
		if role == "" {
			return core.Message{}, core.NewInvalidRequestError(fmt.Sprintf("invalid responses input item at index %d: role is required", index), nil)
		}

		content, ok := ConvertResponsesContentToChatContent(v["content"])
		if !ok {
			return core.Message{}, core.NewInvalidRequestError(fmt.Sprintf("invalid responses input item at index %d: unsupported content", index), nil)
		}
		return core.Message{Role: role, Content: content}, nil
	case core.ResponsesInputItem:
		role := strings.TrimSpace(v.Role)
		if role == "" {
			return core.Message{}, core.NewInvalidRequestError(fmt.Sprintf("invalid responses input item at index %d: role is required", index), nil)
		}

		content, ok := ConvertResponsesContentToChatContent(v.Content)
		if !ok {
			return core.Message{}, core.NewInvalidRequestError(fmt.Sprintf("invalid responses input item at index %d: unsupported content", index), nil)
		}
		return core.Message{Role: role, Content: content}, nil
	default:
		return core.Message{}, core.NewInvalidRequestError(fmt.Sprintf("invalid responses input item at index %d: expected object", index), nil)
	}
}

// ConvertResponsesContentToChatContent maps Responses input content to Chat content.
// Text-only arrays are flattened to strings for broader provider compatibility.
// Any non-text part preserves the array form so multimodal payloads survive routing.
func ConvertResponsesContentToChatContent(content interface{}) (any, bool) {
	switch c := content.(type) {
	case string:
		return c, true
	case []interface{}:
		parts := make([]core.ContentPart, 0, len(c))
		for _, part := range c {
			partMap, ok := part.(map[string]interface{})
			if !ok {
				return nil, false
			}

			partType, _ := partMap["type"].(string)
			switch partType {
			case "text", "input_text":
				text, ok := partMap["text"].(string)
				if !ok || text == "" {
					return nil, false
				}
				parts = append(parts, core.ContentPart{
					Type: "text",
					Text: text,
				})
			case "image_url", "input_image":
				imageURL, detail, mediaType, ok := extractResponsesImageURL(partMap["image_url"])
				if !ok {
					return nil, false
				}
				parts = append(parts, core.ContentPart{
					Type: "image_url",
					ImageURL: &core.ImageURLContent{
						URL:       imageURL,
						Detail:    detail,
						MediaType: mediaType,
					},
				})
			case "input_audio":
				data, format, ok := extractResponsesInputAudio(partMap["input_audio"])
				if !ok {
					return nil, false
				}
				parts = append(parts, core.ContentPart{
					Type: "input_audio",
					InputAudio: &core.InputAudioContent{
						Data:   data,
						Format: format,
					},
				})
			default:
				return nil, false
			}
		}
		return finalizeResponsesChatContent(parts)
	case []core.ResponsesContentPart:
		parts := make([]core.ContentPart, 0, len(c))
		for _, part := range c {
			normalized, ok := normalizeTypedResponsesContentPart(part)
			if !ok {
				return nil, false
			}
			parts = append(parts, normalized)
		}
		return finalizeResponsesChatContent(parts)
	case core.ResponsesContentPart:
		normalized, ok := normalizeTypedResponsesContentPart(c)
		if !ok {
			return nil, false
		}
		return finalizeResponsesChatContent([]core.ContentPart{normalized})
	case []core.ContentPart:
		normalized, err := core.NormalizeMessageContent(c)
		if err != nil {
			return nil, false
		}
		parts, ok := normalized.([]core.ContentPart)
		if !ok {
			return nil, false
		}
		return finalizeResponsesChatContent(parts)
	}
	return nil, false
}

func normalizeTypedResponsesContentPart(part core.ResponsesContentPart) (core.ContentPart, bool) {
	switch part.Type {
	case "text", "input_text":
		if part.Text == "" {
			return core.ContentPart{}, false
		}
		return core.ContentPart{
			Type: "text",
			Text: part.Text,
		}, true
	case "image_url", "input_image":
		if part.ImageURL == nil || part.ImageURL.URL == "" {
			return core.ContentPart{}, false
		}
		return core.ContentPart{
			Type: "image_url",
			ImageURL: &core.ImageURLContent{
				URL:       part.ImageURL.URL,
				Detail:    part.ImageURL.Detail,
				MediaType: part.ImageURL.MediaType,
			},
		}, true
	case "input_audio":
		if part.InputAudio == nil || part.InputAudio.Data == "" || part.InputAudio.Format == "" {
			return core.ContentPart{}, false
		}
		return core.ContentPart{
			Type: "input_audio",
			InputAudio: &core.InputAudioContent{
				Data:   part.InputAudio.Data,
				Format: part.InputAudio.Format,
			},
		}, true
	default:
		return core.ContentPart{}, false
	}
}

func finalizeResponsesChatContent(parts []core.ContentPart) (any, bool) {
	if len(parts) == 0 {
		return nil, false
	}

	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Type != "text" {
			return parts, true
		}
		texts = append(texts, part.Text)
	}
	return strings.Join(texts, " "), true
}

func extractResponsesImageURL(value interface{}) (url string, detail string, mediaType string, ok bool) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return "", "", "", false
		}
		return v, "", "", true
	case map[string]string:
		url = v["url"]
		detail = v["detail"]
		mediaType = v["media_type"]
		return url, detail, mediaType, url != ""
	case map[string]interface{}:
		url, _ = v["url"].(string)
		detail, _ = v["detail"].(string)
		mediaType, _ = v["media_type"].(string)
		return url, detail, mediaType, url != ""
	default:
		return "", "", "", false
	}
}

func extractResponsesInputAudio(value interface{}) (data string, format string, ok bool) {
	switch v := value.(type) {
	case map[string]string:
		data = v["data"]
		format = v["format"]
		return data, format, data != "" && format != ""
	case map[string]interface{}:
		data, _ = v["data"].(string)
		format, _ = v["format"].(string)
		return data, format, data != "" && format != ""
	default:
		return "", "", false
	}
}

// ConvertChatResponseToResponses converts a ChatResponse to a ResponsesResponse.
func ConvertChatResponseToResponses(resp *core.ChatResponse) *core.ResponsesResponse {
	content := ""
	if len(resp.Choices) > 0 {
		content = core.ExtractTextContent(resp.Choices[0].Message.Content)
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
			InputTokens:             resp.Usage.PromptTokens,
			OutputTokens:            resp.Usage.CompletionTokens,
			TotalTokens:             resp.Usage.TotalTokens,
			PromptTokensDetails:     resp.Usage.PromptTokensDetails,
			CompletionTokensDetails: resp.Usage.CompletionTokensDetails,
			RawUsage:                resp.Usage.RawUsage,
		},
	}
}

// ResponsesViaChat implements the Responses API by converting to/from Chat format.
func ResponsesViaChat(ctx context.Context, p ChatProvider, req *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	chatReq, err := ConvertResponsesRequestToChat(req)
	if err != nil {
		return nil, err
	}

	chatResp, err := p.ChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	return ConvertChatResponseToResponses(chatResp), nil
}

// StreamResponsesViaChat implements streaming Responses API by converting to/from Chat format.
func StreamResponsesViaChat(ctx context.Context, p ChatProvider, req *core.ResponsesRequest, providerName string) (io.ReadCloser, error) {
	chatReq, err := ConvertResponsesRequestToChat(req)
	if err != nil {
		return nil, err
	}

	stream, err := p.StreamChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	return NewOpenAIResponsesStreamConverter(stream, req.Model, providerName), nil
}
