package azure

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"gomodel/internal/core"
	"gomodel/internal/llmclient"
)

func TestChatCompletion_UsesAzureAuthAndDefaultAPIVersion(t *testing.T) {
	var gotPath string
	var gotAPIVersion string
	var gotAPIKey string
	var gotAuthorization string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIVersion = r.URL.Query().Get("api-version")
		gotAPIKey = r.Header.Get("api-key")
		gotAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "gpt-4o",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "hello"},
				"finish_reason": "stop"
			}]
		}`))
	}))
	defer server.Close()

	provider := NewWithHTTPClient("test-api-key", server.Client(), llmclient.Hooks{})
	provider.SetBaseURL(server.URL)

	_, err := provider.ChatCompletion(context.Background(), &core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/chat/completions" {
		t.Fatalf("path = %q, want /chat/completions", gotPath)
	}
	if gotAPIKey != "test-api-key" {
		t.Fatalf("api-key = %q, want test-api-key", gotAPIKey)
	}
	if gotAuthorization != "" {
		t.Fatalf("authorization = %q, want empty", gotAuthorization)
	}
	if gotAPIVersion != defaultAPIVersion {
		t.Fatalf("api-version = %q, want %q", gotAPIVersion, defaultAPIVersion)
	}
}

func TestSetAPIVersion_OverridesDefault(t *testing.T) {
	var gotAPIVersion string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIVersion = r.URL.Query().Get("api-version")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"created": 1677652288,
			"model": "gpt-4o",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "hello"},
				"finish_reason": "stop"
			}]
		}`))
	}))
	defer server.Close()

	provider := NewWithHTTPClient("test-api-key", server.Client(), llmclient.Hooks{})
	provider.SetBaseURL(server.URL)
	provider.SetAPIVersion("2025-04-01-preview")

	_, err := provider.ChatCompletion(context.Background(), &core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAPIVersion != "2025-04-01-preview" {
		t.Fatalf("api-version = %q, want 2025-04-01-preview", gotAPIVersion)
	}
}
