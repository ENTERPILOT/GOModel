package core

import (
	"fmt"
	"net/http"
	neturl "net/url"
	"strings"
)

// DecodedBatchItemRequest is the canonical decode result for known JSON batch subrequests.
type DecodedBatchItemRequest struct {
	Endpoint  string
	Method    string
	Operation string
	Request   any
}

func (decoded *DecodedBatchItemRequest) ChatRequest() *ChatRequest {
	if decoded == nil {
		return nil
	}
	req, _ := decoded.Request.(*ChatRequest)
	return req
}

func (decoded *DecodedBatchItemRequest) ResponsesRequest() *ResponsesRequest {
	if decoded == nil {
		return nil
	}
	req, _ := decoded.Request.(*ResponsesRequest)
	return req
}

func (decoded *DecodedBatchItemRequest) EmbeddingRequest() *EmbeddingRequest {
	if decoded == nil {
		return nil
	}
	req, _ := decoded.Request.(*EmbeddingRequest)
	return req
}

func (decoded *DecodedBatchItemRequest) ModelSelector() (ModelSelector, error) {
	if decoded == nil {
		return ModelSelector{}, fmt.Errorf("decoded batch request is required")
	}

	switch decoded.Operation {
	case "chat_completions":
		req := decoded.ChatRequest()
		if req == nil {
			return ModelSelector{}, fmt.Errorf("missing chat request")
		}
		return ParseModelSelector(req.Model, req.Provider)
	case "responses":
		req := decoded.ResponsesRequest()
		if req == nil {
			return ModelSelector{}, fmt.Errorf("missing responses request")
		}
		return ParseModelSelector(req.Model, req.Provider)
	case "embeddings":
		req := decoded.EmbeddingRequest()
		if req == nil {
			return ModelSelector{}, fmt.Errorf("missing embeddings request")
		}
		return ParseModelSelector(req.Model, req.Provider)
	default:
		return ModelSelector{}, fmt.Errorf("unsupported batch item url: %s", decoded.Endpoint)
	}
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
		req, err := unmarshalCanonicalJSON(item.Body, func() *ChatRequest { return &ChatRequest{} })
		if err != nil {
			return nil, fmt.Errorf("invalid chat request body: %w", err)
		}
		decoded.Request = req
	case "responses":
		req, err := unmarshalCanonicalJSON(item.Body, func() *ResponsesRequest { return &ResponsesRequest{} })
		if err != nil {
			return nil, fmt.Errorf("invalid responses request body: %w", err)
		}
		decoded.Request = req
	case "embeddings":
		req, err := unmarshalCanonicalJSON(item.Body, func() *EmbeddingRequest { return &EmbeddingRequest{} })
		if err != nil {
			return nil, fmt.Errorf("invalid embeddings request body: %w", err)
		}
		decoded.Request = req
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
	return decoded.ModelSelector()
}
