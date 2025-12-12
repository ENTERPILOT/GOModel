//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gomodel/internal/core"
)

func TestResponses(t *testing.T) {
	t.Run("basic string input", func(t *testing.T) {
		payload := core.ResponsesRequest{
			Model: "gpt-4.1",
			Input: "What is the capital of France?",
		}

		resp := sendResponsesRequest(t, payload)
		defer closeBody(resp)

		require.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK for basic request")

		var respBody core.ResponsesResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBody))

		assert.NotEmpty(t, respBody.ID)
		assert.Equal(t, "response", respBody.Object)
		assert.Equal(t, "gpt-4.1", respBody.Model)
		assert.Equal(t, "completed", respBody.Status)
		assert.NotEmpty(t, respBody.Output)

		if len(respBody.Output) > 0 {
			assert.Equal(t, "message", respBody.Output[0].Type)
			assert.Equal(t, "assistant", respBody.Output[0].Role)
		}
	})

	t.Run("with instructions", func(t *testing.T) {
		payload := core.ResponsesRequest{
			Model:        "gpt-4.1",
			Input:        "Tell me about Go programming",
			Instructions: "You are a helpful programming assistant.",
		}

		resp := sendResponsesRequest(t, payload)
		defer closeBody(resp)

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var respBody core.ResponsesResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBody))
		assert.Equal(t, "completed", respBody.Status)
	})

	t.Run("array input conversation", func(t *testing.T) {
		payload := core.ResponsesRequest{
			Model: "gpt-4.1",
			Input: []map[string]interface{}{
				{"role": "user", "content": "What is 2 + 2?"},
				{"role": "assistant", "content": "2 + 2 equals 4."},
				{"role": "user", "content": "And what is 3 + 3?"},
			},
		}

		resp := sendResponsesRequest(t, payload)
		defer closeBody(resp)

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var respBody core.ResponsesResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBody))
		assert.Equal(t, "completed", respBody.Status)
	})
}

func TestResponsesParameters(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*core.ResponsesRequest)
	}{
		{
			name: "with temperature",
			modify: func(r *core.ResponsesRequest) {
				temp := 0.5
				r.Temperature = &temp
			},
		},
		{
			name: "with max_output_tokens",
			modify: func(r *core.ResponsesRequest) {
				maxTokens := 100
				r.MaxOutputTokens = &maxTokens
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := core.ResponsesRequest{
				Model: "gpt-4.1",
				Input: "Hello",
			}
			tt.modify(&payload)

			resp := sendResponsesRequest(t, payload)
			defer closeBody(resp)

			require.Equal(t, http.StatusOK, resp.StatusCode)

			var respBody core.ResponsesResponse
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBody))
			assert.Equal(t, "completed", respBody.Status)
		})
	}
}

func TestResponsesStreaming(t *testing.T) {
	t.Run("basic streaming", func(t *testing.T) {
		payload := core.ResponsesRequest{
			Model:  "gpt-4.1",
			Input:  "Count from 1 to 5",
			Stream: true,
		}

		resp := sendResponsesRequest(t, payload)
		defer closeBody(resp)

		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

		events := readResponsesStream(t, resp.Body)
		require.Greater(t, len(events), 0)
		assert.True(t, hasDoneEvent(events), "Should receive done event")
	})

	t.Run("streaming content", func(t *testing.T) {
		payload := core.ResponsesRequest{
			Model:  "gpt-4.1",
			Input:  "Hello",
			Stream: true,
		}

		resp := sendResponsesRequest(t, payload)
		defer closeBody(resp)

		require.Equal(t, http.StatusOK, resp.StatusCode)

		events := readResponsesStream(t, resp.Body)
		content := extractResponsesStreamContent(events)
		assert.NotEmpty(t, content)
	})
}

func TestResponsesTools(t *testing.T) {
	tests := []struct {
		name  string
		tools []map[string]interface{}
	}{
		{
			name: "file_search tool",
			tools: []map[string]interface{}{
				{"type": "file_search", "vector_store_ids": []string{"vs_test"}},
			},
		},
		{
			name: "web_search tool",
			tools: []map[string]interface{}{
				{"type": "web_search_preview"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := core.ResponsesRequest{
				Model: "gpt-4.1",
				Input: "Search for information",
				Tools: tt.tools,
			}

			resp := sendResponsesRequest(t, payload)
			defer closeBody(resp)

			// Tools may or may not be supported
			assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest,
				"Expected OK or BadRequest, got %d", resp.StatusCode)
		})
	}
}

func TestResponsesErrors(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		resp, err := http.Post(gatewayURL+responsesPath, "application/json",
			strings.NewReader(`{"model": "gpt-4.1", "input": invalid}`))
		require.NoError(t, err)
		defer closeBody(resp)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("missing model", func(t *testing.T) {
		resp := sendRawResponsesRequest(t, map[string]interface{}{"input": "Hello"})
		defer closeBody(resp)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("empty input", func(t *testing.T) {
		payload := core.ResponsesRequest{Model: "gpt-4.1", Input: ""}

		resp := sendResponsesRequest(t, payload)
		defer closeBody(resp)

		// Empty input should be handled gracefully
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest)
	})

	t.Run("invalid model", func(t *testing.T) {
		payload := core.ResponsesRequest{Model: "invalid-model-xyz", Input: "Hello"}

		resp := sendResponsesRequest(t, payload)
		defer closeBody(resp)

		// Accept either error or pass-through
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest)
	})
}

func TestResponsesUsage(t *testing.T) {
	payload := core.ResponsesRequest{
		Model: "gpt-4.1",
		Input: "Hello, how are you?",
	}

	resp := sendResponsesRequest(t, payload)
	defer closeBody(resp)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var respBody core.ResponsesResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBody))

	if respBody.Usage != nil {
		assert.Greater(t, respBody.Usage.InputTokens, 0)
		assert.Greater(t, respBody.Usage.OutputTokens, 0)
		assert.Equal(t, respBody.Usage.InputTokens+respBody.Usage.OutputTokens, respBody.Usage.TotalTokens)
	}
}

func TestResponsesMultimodal(t *testing.T) {
	payload := core.ResponsesRequest{
		Model: "gpt-4.1",
		Input: []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "input_text", "text": "What's in this image?"},
					{"type": "input_image", "image_url": map[string]string{"url": "https://example.com/image.jpg"}},
				},
			},
		},
	}

	resp := sendResponsesRequest(t, payload)
	defer closeBody(resp)

	// Multimodal may or may not be supported
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest)
}

func TestResponsesConcurrency(t *testing.T) {
	const numRequests = 5
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(idx int) {
			payload := core.ResponsesRequest{
				Model: "gpt-4.1",
				Input: "Quick test " + string(rune('A'+idx)),
			}

			resp := sendResponsesRequest(t, payload)
			defer closeBody(resp)
			results <- resp.StatusCode
		}(i)
	}

	successCount := 0
	for i := 0; i < numRequests; i++ {
		select {
		case status := <-results:
			if status == http.StatusOK {
				successCount++
			}
		case <-time.After(30 * time.Second):
			t.Fatal("Timeout waiting for concurrent requests")
		}
	}

	assert.Equal(t, numRequests, successCount)
}



