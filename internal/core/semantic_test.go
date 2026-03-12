package core

import (
	"net/http"
	"testing"
)

func TestBuildSemanticEnvelope_OpenAICompat(t *testing.T) {
	frame := NewRequestSnapshot(
		"POST",
		"/v1/chat/completions",
		nil,
		nil,
		nil,
		"application/json",
		[]byte(`{
			"model":"gpt-5-mini",
			"provider":"openai",
			"messages":[{"role":"user","content":"hello"}],
			"response_format":{"type":"json_schema"}
		}`),
		false,
		"",
		nil,
	)

	env := DeriveRequestSemantics(frame)
	if env == nil {
		t.Fatal("DeriveRequestSemantics() = nil")
		return
	}
	if env.RouteKind != "openai_compat" {
		t.Fatalf("RouteKind = %q, want openai_compat", env.RouteKind)
	}
	if env.OperationKind != "chat_completions" {
		t.Fatalf("OperationKind = %q, want chat_completions", env.OperationKind)
	}
	if !env.BodyParsedAsJSON {
		t.Fatal("BodyParsedAsJSON = false, want true")
	}
	if env.RoutingHints.Model != "gpt-5-mini" {
		t.Fatalf("RoutingHints.Model = %q, want gpt-5-mini", env.RoutingHints.Model)
	}
	if env.RoutingHints.Provider != "openai" {
		t.Fatalf("RoutingHints.Provider = %q, want openai", env.RoutingHints.Provider)
	}
	if env.CachedChatRequest() != nil || env.CachedResponsesRequest() != nil || env.CachedEmbeddingRequest() != nil || env.CachedBatchRequest() != nil || env.CachedBatchRouteInfo() != nil || env.CachedFileRouteInfo() != nil {
		t.Fatalf("canonical request payloads should be nil, got %+v", env)
	}
}

func TestBuildSemanticEnvelope_InvalidJSONRemainsPartial(t *testing.T) {
	frame := NewRequestSnapshot("POST", "/v1/responses", nil, nil, nil, "application/json", []byte(`{invalid}`), false, "", nil)

	env := DeriveRequestSemantics(frame)
	if env == nil {
		t.Fatal("DeriveRequestSemantics() = nil")
		return
	}
	if env.RouteKind != "openai_compat" {
		t.Fatalf("RouteKind = %q, want openai_compat", env.RouteKind)
	}
	if env.OperationKind != "responses" {
		t.Fatalf("OperationKind = %q, want responses", env.OperationKind)
	}
	if env.BodyParsedAsJSON {
		t.Fatal("BodyParsedAsJSON = true, want false")
	}
	if env.RoutingHints.Model != "" {
		t.Fatalf("RoutingHints.Model = %q, want empty", env.RoutingHints.Model)
	}
}

func TestBuildSemanticEnvelope_PassthroughRouteParams(t *testing.T) {
	frame := NewRequestSnapshot(
		"POST",
		"/p/openai/responses",
		map[string]string{"provider": "openai", "endpoint": "responses"},
		nil,
		nil,
		"",
		[]byte(`{"model":"gpt-5-mini","foo":"bar"}`),
		false,
		"",
		nil,
	)

	env := DeriveRequestSemantics(frame)
	if env == nil {
		t.Fatal("DeriveRequestSemantics() = nil")
		return
	}
	if env.RouteKind != "provider_passthrough" {
		t.Fatalf("RouteKind = %q, want provider_passthrough", env.RouteKind)
	}
	if env.OperationKind != "provider_passthrough" {
		t.Fatalf("OperationKind = %q, want provider_passthrough", env.OperationKind)
	}
	if env.RoutingHints.Provider != "openai" {
		t.Fatalf("RoutingHints.Provider = %q, want openai", env.RoutingHints.Provider)
	}
	if env.RoutingHints.Endpoint != "responses" {
		t.Fatalf("RoutingHints.Endpoint = %q, want responses", env.RoutingHints.Endpoint)
	}
	if env.RoutingHints.Model != "gpt-5-mini" {
		t.Fatalf("RoutingHints.Model = %q, want gpt-5-mini", env.RoutingHints.Model)
	}
	if env.CachedChatRequest() != nil || env.CachedResponsesRequest() != nil || env.CachedEmbeddingRequest() != nil || env.CachedBatchRequest() != nil || env.CachedBatchRouteInfo() != nil || env.CachedFileRouteInfo() != nil {
		t.Fatalf("canonical request payloads should be nil, got %+v", env)
	}
}

func TestBuildSemanticEnvelope_PassthroughPathFallback(t *testing.T) {
	frame := NewRequestSnapshot("POST", "/p/anthropic/messages", nil, nil, nil, "", []byte(`{"model":"claude-sonnet-4-5"}`), false, "", nil)

	env := DeriveRequestSemantics(frame)
	if env == nil {
		t.Fatal("DeriveRequestSemantics() = nil")
		return
	}
	if env.RoutingHints.Provider != "anthropic" {
		t.Fatalf("RoutingHints.Provider = %q, want anthropic", env.RoutingHints.Provider)
	}
	if env.RoutingHints.Endpoint != "messages" {
		t.Fatalf("RoutingHints.Endpoint = %q, want messages", env.RoutingHints.Endpoint)
	}
}

func TestBuildSemanticEnvelope_SkipsBodyParsingWhenIngressBodyWasNotCaptured(t *testing.T) {
	frame := NewRequestSnapshot("POST", "/v1/chat/completions", nil, nil, nil, "", nil, true, "", nil)

	env := DeriveRequestSemantics(frame)
	if env == nil {
		t.Fatal("DeriveRequestSemantics() = nil")
		return
	}
	if env.BodyParsedAsJSON {
		t.Fatal("BodyParsedAsJSON = true, want false")
	}
	if env.RoutingHints.Model != "" {
		t.Fatalf("RoutingHints.Model = %q, want empty", env.RoutingHints.Model)
	}
}

func TestBuildSemanticEnvelope_FilesMetadata(t *testing.T) {
	frame := NewRequestSnapshot(
		"GET",
		"/v1/files/file_123/content",
		map[string]string{"id": "file_123"},
		map[string][]string{
			"provider": {"openai"},
		},
		nil,
		"application/octet-stream",
		nil,
		false,
		"",
		nil,
	)

	env := DeriveRequestSemantics(frame)
	if env == nil {
		t.Fatal("DeriveRequestSemantics() = nil")
		return
	}
	if env.OperationKind != "files" {
		t.Fatalf("OperationKind = %q, want files", env.OperationKind)
	}
	req := env.CachedFileRouteInfo()
	if req == nil {
		t.Fatal("FileRequest = nil")
		return
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
	if env.RoutingHints.Provider != "openai" {
		t.Fatalf("RoutingHints.Provider = %q, want openai", env.RoutingHints.Provider)
	}
}

func TestBuildSemanticEnvelope_BatchesListMetadata(t *testing.T) {
	frame := NewRequestSnapshot(
		http.MethodGet,
		"/v1/batches",
		nil,
		map[string][]string{
			"after": {"batch_prev"},
			"limit": {"5"},
		},
		nil,
		"",
		nil,
		false,
		"",
		nil,
	)

	env := DeriveRequestSemantics(frame)
	if env == nil {
		t.Fatal("DeriveRequestSemantics() = nil")
		return
	}
	if env.OperationKind != "batches" {
		t.Fatalf("OperationKind = %q, want batches", env.OperationKind)
	}
	req := env.CachedBatchRouteInfo()
	if req == nil {
		t.Fatal("BatchMetadata = nil")
		return
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
	frame := NewRequestSnapshot(http.MethodGet, "/v1/batches/batch_123/results", map[string]string{"id": "batch_123"}, nil, nil, "", nil, false, "", nil)

	env := DeriveRequestSemantics(frame)
	if env == nil {
		t.Fatal("DeriveRequestSemantics() = nil")
		return
	}
	if env.OperationKind != "batches" {
		t.Fatalf("OperationKind = %q, want batches", env.OperationKind)
	}
	req := env.CachedBatchRouteInfo()
	if req == nil {
		t.Fatal("BatchMetadata = nil")
		return
	}
	if req.Action != BatchActionResults {
		t.Fatalf("BatchMetadata.Action = %q, want %q", req.Action, BatchActionResults)
	}
	if req.BatchID != "batch_123" {
		t.Fatalf("BatchMetadata.BatchID = %q, want batch_123", req.BatchID)
	}
}
