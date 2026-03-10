package core

import (
	"net/http"
	"testing"
)

func TestBuildSemanticEnvelope_OpenAICompat(t *testing.T) {
	frame := &IngressFrame{
		Method:      "POST",
		Path:        "/v1/chat/completions",
		ContentType: "application/json",
		RawBody: []byte(`{
			"model":"gpt-5-mini",
			"provider":"openai",
			"messages":[{"role":"user","content":"hello"}],
			"response_format":{"type":"json_schema"}
		}`),
	}

	env := BuildSemanticEnvelope(frame)
	if env == nil {
		t.Fatal("BuildSemanticEnvelope() = nil")
	}
	if env.Dialect != "openai_compat" {
		t.Fatalf("Dialect = %q, want openai_compat", env.Dialect)
	}
	if env.Operation != "chat_completions" {
		t.Fatalf("Operation = %q, want chat_completions", env.Operation)
	}
	if !env.JSONBodyParsed {
		t.Fatal("JSONBodyParsed = false, want true")
	}
	if env.SelectorHints.Model != "gpt-5-mini" {
		t.Fatalf("SelectorHints.Model = %q, want gpt-5-mini", env.SelectorHints.Model)
	}
	if env.SelectorHints.Provider != "openai" {
		t.Fatalf("SelectorHints.Provider = %q, want openai", env.SelectorHints.Provider)
	}
	if env.CachedChatRequest() != nil || env.CachedResponsesRequest() != nil || env.CachedEmbeddingRequest() != nil || env.CachedBatchRequest() != nil || env.CachedBatchMetadata() != nil || env.CachedFileRequest() != nil {
		t.Fatalf("canonical request payloads should be nil, got %+v", env)
	}
}

func TestBuildSemanticEnvelope_InvalidJSONRemainsPartial(t *testing.T) {
	frame := &IngressFrame{
		Method:      "POST",
		Path:        "/v1/responses",
		ContentType: "application/json",
		RawBody:     []byte(`{invalid}`),
	}

	env := BuildSemanticEnvelope(frame)
	if env == nil {
		t.Fatal("BuildSemanticEnvelope() = nil")
	}
	if env.Dialect != "openai_compat" {
		t.Fatalf("Dialect = %q, want openai_compat", env.Dialect)
	}
	if env.Operation != "responses" {
		t.Fatalf("Operation = %q, want responses", env.Operation)
	}
	if env.JSONBodyParsed {
		t.Fatal("JSONBodyParsed = true, want false")
	}
	if env.SelectorHints.Model != "" {
		t.Fatalf("SelectorHints.Model = %q, want empty", env.SelectorHints.Model)
	}
}

func TestBuildSemanticEnvelope_PassthroughRouteParams(t *testing.T) {
	frame := &IngressFrame{
		Method:      "POST",
		Path:        "/p/openai/responses",
		RouteParams: map[string]string{"provider": "openai", "endpoint": "responses"},
		RawBody:     []byte(`{"model":"gpt-5-mini","foo":"bar"}`),
	}

	env := BuildSemanticEnvelope(frame)
	if env == nil {
		t.Fatal("BuildSemanticEnvelope() = nil")
	}
	if env.Dialect != "provider_passthrough" {
		t.Fatalf("Dialect = %q, want provider_passthrough", env.Dialect)
	}
	if env.Operation != "provider_passthrough" {
		t.Fatalf("Operation = %q, want provider_passthrough", env.Operation)
	}
	if env.SelectorHints.Provider != "openai" {
		t.Fatalf("SelectorHints.Provider = %q, want openai", env.SelectorHints.Provider)
	}
	if env.SelectorHints.Endpoint != "responses" {
		t.Fatalf("SelectorHints.Endpoint = %q, want responses", env.SelectorHints.Endpoint)
	}
	if env.SelectorHints.Model != "gpt-5-mini" {
		t.Fatalf("SelectorHints.Model = %q, want gpt-5-mini", env.SelectorHints.Model)
	}
	if env.CachedChatRequest() != nil || env.CachedResponsesRequest() != nil || env.CachedEmbeddingRequest() != nil || env.CachedBatchRequest() != nil || env.CachedBatchMetadata() != nil || env.CachedFileRequest() != nil {
		t.Fatalf("canonical request payloads should be nil, got %+v", env)
	}
}

func TestBuildSemanticEnvelope_PassthroughPathFallback(t *testing.T) {
	frame := &IngressFrame{
		Method:  "POST",
		Path:    "/p/anthropic/messages",
		RawBody: []byte(`{"model":"claude-sonnet-4-5"}`),
	}

	env := BuildSemanticEnvelope(frame)
	if env == nil {
		t.Fatal("BuildSemanticEnvelope() = nil")
	}
	if env.SelectorHints.Provider != "anthropic" {
		t.Fatalf("SelectorHints.Provider = %q, want anthropic", env.SelectorHints.Provider)
	}
	if env.SelectorHints.Endpoint != "messages" {
		t.Fatalf("SelectorHints.Endpoint = %q, want messages", env.SelectorHints.Endpoint)
	}
}

func TestBuildSemanticEnvelope_SkipsBodyParsingWhenIngressBodyWasNotCaptured(t *testing.T) {
	frame := &IngressFrame{
		Method:          "POST",
		Path:            "/v1/chat/completions",
		RawBodyTooLarge: true,
	}

	env := BuildSemanticEnvelope(frame)
	if env == nil {
		t.Fatal("BuildSemanticEnvelope() = nil")
	}
	if env.JSONBodyParsed {
		t.Fatal("JSONBodyParsed = true, want false")
	}
	if env.SelectorHints.Model != "" {
		t.Fatalf("SelectorHints.Model = %q, want empty", env.SelectorHints.Model)
	}
}

func TestBuildSemanticEnvelope_FilesMetadata(t *testing.T) {
	frame := &IngressFrame{
		Method:      "GET",
		Path:        "/v1/files/file_123/content",
		RouteParams: map[string]string{"id": "file_123"},
		QueryParams: map[string][]string{
			"provider": {"openai"},
		},
		ContentType: "application/octet-stream",
	}

	env := BuildSemanticEnvelope(frame)
	if env == nil {
		t.Fatal("BuildSemanticEnvelope() = nil")
	}
	if env.Operation != "files" {
		t.Fatalf("Operation = %q, want files", env.Operation)
	}
	req := env.CachedFileRequest()
	if req == nil {
		t.Fatal("FileRequest = nil")
	}
	if req.Action != FileActionContent {
		t.Fatalf("FileRequest.Action = %q, want %q", req.Action, FileActionContent)
	}
	if req.FileID != "file_123" {
		t.Fatalf("FileRequest.FileID = %q, want file_123", req.FileID)
	}
	if req.Provider != "openai" {
		t.Fatalf("FileRequest.Provider = %q, want openai", req.Provider)
	}
	if env.SelectorHints.Provider != "openai" {
		t.Fatalf("SelectorHints.Provider = %q, want openai", env.SelectorHints.Provider)
	}
}

func TestBuildSemanticEnvelope_BatchesListMetadata(t *testing.T) {
	frame := &IngressFrame{
		Method: http.MethodGet,
		Path:   "/v1/batches",
		QueryParams: map[string][]string{
			"after": {"batch_prev"},
			"limit": {"5"},
		},
	}

	env := BuildSemanticEnvelope(frame)
	if env == nil {
		t.Fatal("BuildSemanticEnvelope() = nil")
	}
	if env.Operation != "batches" {
		t.Fatalf("Operation = %q, want batches", env.Operation)
	}
	req := env.CachedBatchMetadata()
	if req == nil {
		t.Fatal("BatchMetadata = nil")
	}
	if req.Action != BatchActionList {
		t.Fatalf("BatchMetadata.Action = %q, want %q", req.Action, BatchActionList)
	}
	if req.After != "batch_prev" {
		t.Fatalf("BatchMetadata.After = %q, want batch_prev", req.After)
	}
	if !req.HasLimit || req.Limit != 5 {
		t.Fatalf("BatchMetadata limit = %d/%v, want 5/true", req.Limit, req.HasLimit)
	}
}

func TestBuildSemanticEnvelope_BatchResultsMetadata(t *testing.T) {
	frame := &IngressFrame{
		Method:      http.MethodGet,
		Path:        "/v1/batches/batch_123/results",
		RouteParams: map[string]string{"id": "batch_123"},
	}

	env := BuildSemanticEnvelope(frame)
	if env == nil {
		t.Fatal("BuildSemanticEnvelope() = nil")
	}
	if env.Operation != "batches" {
		t.Fatalf("Operation = %q, want batches", env.Operation)
	}
	req := env.CachedBatchMetadata()
	if req == nil {
		t.Fatal("BatchMetadata = nil")
	}
	if req.Action != BatchActionResults {
		t.Fatalf("BatchMetadata.Action = %q, want %q", req.Action, BatchActionResults)
	}
	if req.BatchID != "batch_123" {
		t.Fatalf("BatchMetadata.BatchID = %q, want batch_123", req.BatchID)
	}
}
