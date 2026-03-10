package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	neturl "net/url"
	"strings"
)

// DecodedBatchItemRequest is the canonical decode result for known JSON batch subrequests.
type DecodedBatchItemRequest struct {
	Endpoint         string
	Method           string
	Operation        string
	ChatRequest      *ChatRequest
	ResponsesRequest *ResponsesRequest
	EmbeddingRequest *EmbeddingRequest
}

// NormalizeOperationPath returns a stable path-only form for model-facing endpoints.
func NormalizeOperationPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if parsed, err := neturl.Parse(trimmed); err == nil && parsed.Path != "" {
		trimmed = parsed.Path
	}
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return "/"
	}
	return trimmed
}

// ResolveBatchItemEndpoint prefers an inline item URL and otherwise falls back to the batch default endpoint.
func ResolveBatchItemEndpoint(defaultEndpoint, itemURL string) string {
	if strings.TrimSpace(itemURL) != "" {
		return itemURL
	}
	return defaultEndpoint
}

// DecodeKnownBatchItemRequest normalizes and decodes a known JSON batch subrequest.
func DecodeKnownBatchItemRequest(defaultEndpoint string, item BatchRequestItem) (*DecodedBatchItemRequest, error) {
	endpoint := NormalizeOperationPath(ResolveBatchItemEndpoint(defaultEndpoint, item.URL))
	if endpoint == "" {
		return nil, fmt.Errorf("url is required")
	}

	method := strings.ToUpper(strings.TrimSpace(item.Method))
	if method == "" {
		method = http.MethodPost
	}
	if method != http.MethodPost {
		return nil, fmt.Errorf("only POST is supported")
	}
	if len(item.Body) == 0 {
		return nil, fmt.Errorf("body is required")
	}

	decoded := &DecodedBatchItemRequest{
		Endpoint:  endpoint,
		Method:    method,
		Operation: DescribeEndpointPath(endpoint).Operation,
	}

	switch decoded.Operation {
	case "chat_completions":
		var req ChatRequest
		if err := json.Unmarshal(item.Body, &req); err != nil {
			return nil, fmt.Errorf("invalid chat request body: %w", err)
		}
		decoded.ChatRequest = &req
	case "responses":
		var req ResponsesRequest
		if err := json.Unmarshal(item.Body, &req); err != nil {
			return nil, fmt.Errorf("invalid responses request body: %w", err)
		}
		decoded.ResponsesRequest = &req
	case "embeddings":
		var req EmbeddingRequest
		if err := json.Unmarshal(item.Body, &req); err != nil {
			return nil, fmt.Errorf("invalid embeddings request body: %w", err)
		}
		decoded.EmbeddingRequest = &req
	default:
		return nil, fmt.Errorf("unsupported batch item url: %s", endpoint)
	}
	return decoded, nil
}

// BatchItemModelSelector derives the model selector for a known JSON batch subrequest.
func BatchItemModelSelector(defaultEndpoint string, item BatchRequestItem) (ModelSelector, error) {
	decoded, err := DecodeKnownBatchItemRequest(defaultEndpoint, item)
	if err != nil {
		return ModelSelector{}, err
	}

	switch decoded.Operation {
	case "chat_completions":
		return ParseModelSelector(decoded.ChatRequest.Model, decoded.ChatRequest.Provider)
	case "responses":
		return ParseModelSelector(decoded.ResponsesRequest.Model, decoded.ResponsesRequest.Provider)
	case "embeddings":
		return ParseModelSelector(decoded.EmbeddingRequest.Model, decoded.EmbeddingRequest.Provider)
	default:
		return ModelSelector{}, fmt.Errorf("unsupported batch item url: %s", decoded.Endpoint)
	}
}
