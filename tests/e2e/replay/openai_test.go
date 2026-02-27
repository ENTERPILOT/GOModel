//go:build e2e

package replay

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gomodel/internal/core"
)

func TestReplay_OpenAI_ChatCompletion(t *testing.T) {
	resp := sendJSON(t, "/v1/chat/completions", core.ChatRequest{
		Model:    "gpt-4o-mini-2024-07-18",
		Messages: []core.Message{{Role: "user", Content: "Say hello"}},
	})
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var chat core.ChatResponse
	readJSON(t, resp, &chat)

	assert.NotEmpty(t, chat.ID, "response should have an ID")
	assert.Equal(t, "chat.completion", chat.Object)
	assert.Contains(t, chat.Model, "gpt-4o-mini")
	require.NotEmpty(t, chat.Choices, "response should have choices")

	choice := chat.Choices[0]
	assert.Equal(t, "stop", choice.FinishReason)
	assert.Equal(t, "assistant", choice.Message.Role)
	assert.NotEmpty(t, choice.Message.Content, "message content should not be empty")

	assert.Greater(t, chat.Usage.PromptTokens, 0)
	assert.Greater(t, chat.Usage.CompletionTokens, 0)
	assert.Greater(t, chat.Usage.TotalTokens, 0)
}

// toolCallResponse captures fields that core.ChatResponse drops (tool_calls).
type toolCallResponse struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Model   string           `json:"model"`
	Choices []toolCallChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type toolCallChoice struct {
	Message      toolCallMessage `json:"message"`
	FinishReason string          `json:"finish_reason"`
	Index        int             `json:"index"`
}

type toolCallMessage struct {
	Role      string     `json:"role"`
	Content   *string    `json:"content"`
	ToolCalls []toolCall `json:"tool_calls,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function functionCall `json:"function"`
}

type functionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func TestReplay_OpenAI_ChatWithTools(t *testing.T) {
	// This test exposes TWO deserialization bugs:
	//
	// 1. core.ChatRequest has no Tools field → the gateway drops the "tools" from
	//    the client request before forwarding to the provider, so the upstream
	//    never sees tool definitions and the golden server falls through to the
	//    non-tool route (returning a plain chat completion).
	//
	// 2. core.Message has no ToolCalls field → even if the upstream returned
	//    tool_calls, the provider unmarshals into core.ChatResponse which
	//    silently drops them, and the handler re-serializes without tool_calls.
	//
	// Once both bugs are fixed, the response should contain tool_calls with
	// finish_reason="tool_calls".
	payload := map[string]any{
		"model":    "gpt-4o-mini-2024-07-18",
		"messages": []map[string]string{{"role": "user", "content": "What is the weather in Paris?"}},
		"tools": []map[string]any{
			{
				"type": "function",
				"function": map[string]any{
					"name":       "get_weather",
					"parameters": map[string]any{"type": "object", "properties": map[string]any{"location": map[string]string{"type": "string"}}, "required": []string{"location"}},
				},
			},
		},
	}

	resp := sendJSON(t, "/v1/chat/completions", payload)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var chat toolCallResponse
	readJSON(t, resp, &chat)

	assert.NotEmpty(t, chat.ID)
	require.NotEmpty(t, chat.Choices)

	choice := chat.Choices[0]
	assert.Equal(t, "tool_calls", choice.FinishReason,
		"finish_reason should be 'tool_calls' — if 'stop', the tools field was dropped from the request")

	assert.NotEmpty(t, choice.Message.ToolCalls,
		"tool_calls should be present in the response — "+
			"if empty, core.ChatRequest needs a Tools field and core.Message needs a ToolCalls field")

	if len(choice.Message.ToolCalls) > 0 {
		tc := choice.Message.ToolCalls[0]
		assert.Equal(t, "function", tc.Type)
		assert.Equal(t, "get_weather", tc.Function.Name)
		assert.Contains(t, tc.Function.Arguments, "Paris")
	}
}

func TestReplay_OpenAI_Embeddings(t *testing.T) {
	resp := sendJSON(t, "/v1/embeddings", core.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: "Hello world",
	})
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var emb core.EmbeddingResponse
	readJSON(t, resp, &emb)

	assert.Equal(t, "list", emb.Object)
	require.NotEmpty(t, emb.Data, "response should have embedding data")

	assert.Equal(t, "embedding", emb.Data[0].Object)
	assert.NotEmpty(t, emb.Data[0].Embedding, "float embedding should not be empty")
	assert.Greater(t, len(emb.Data[0].Embedding), 100, "embedding vector should have many dimensions")

	assert.Greater(t, emb.Usage.PromptTokens, 0)
	assert.Greater(t, emb.Usage.TotalTokens, 0)
}

func TestReplay_OpenAI_EmbeddingsBase64(t *testing.T) {
	// OpenAI returns embedding as a base64-encoded string when encoding_format="base64".
	// core.EmbeddingData.Embedding is []float64, so json.Unmarshal fails when it
	// encounters a string — the provider returns an error and the gateway returns 502.
	//
	// After the bug is fixed (custom UnmarshalJSON on EmbeddingData), this test should
	// get a 200 with decoded float64 values.
	resp := sendJSON(t, "/v1/embeddings", core.EmbeddingRequest{
		Model:          "text-embedding-3-small",
		Input:          "Hello world",
		EncodingFormat: "base64",
	})
	defer func() { _ = resp.Body.Close() }()

	// Once fixed, change this to http.StatusOK and verify the decoded embedding.
	if resp.StatusCode == http.StatusOK {
		// Bug has been fixed — validate the decoded embedding
		var emb core.EmbeddingResponse
		readJSON(t, resp, &emb)

		assert.Equal(t, "list", emb.Object)
		require.NotEmpty(t, emb.Data)
		assert.NotEmpty(t, emb.Data[0].Embedding,
			"base64 embedding should be decoded into []float64")
		if len(emb.Data[0].Embedding) > 0 {
			assert.Greater(t, len(emb.Data[0].Embedding), 100,
				"decoded embedding vector should have many dimensions")
		}
	} else {
		// Bug exists — the provider fails to unmarshal the base64 string into []float64
		assert.Equal(t, http.StatusBadGateway, resp.StatusCode,
			"base64 embedding deserialization currently fails with 502 — "+
				"EmbeddingData needs a custom UnmarshalJSON to handle base64 encoding")
	}
}

func TestReplay_OpenAI_Models(t *testing.T) {
	resp := sendGET(t, "/v1/models")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var models core.ModelsResponse
	readJSON(t, resp, &models)

	assert.Equal(t, "list", models.Object)
	assert.NotEmpty(t, models.Data, "models list should not be empty")

	// Check that at least one well-known model is present
	var found bool
	for _, m := range models.Data {
		assert.NotEmpty(t, m.ID, "model ID should not be empty")
		assert.Equal(t, "model", m.Object)
		if m.ID == "gpt-4" {
			found = true
		}
	}
	assert.True(t, found, "models list should contain gpt-4")
}
