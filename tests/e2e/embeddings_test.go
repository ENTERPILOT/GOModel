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

func TestEmbeddings(t *testing.T) {
	t.Run("basic string input", func(t *testing.T) {
		payload := core.EmbeddingRequest{
			Model: "text-embedding-3-small",
			Input: "Hello, world!",
		}

		resp := sendEmbeddingsRequest(t, payload)
		defer closeBody(resp)

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var embResp core.EmbeddingResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&embResp))

		assert.Equal(t, "list", embResp.Object)
		require.Len(t, embResp.Data, 1)
		assert.Equal(t, "embedding", embResp.Data[0].Object)
		assert.Equal(t, 0, embResp.Data[0].Index)
		assert.NotEmpty(t, embResp.Data[0].Embedding)
	})

	t.Run("array input", func(t *testing.T) {
		payload := core.EmbeddingRequest{
			Model: "text-embedding-3-small",
			Input: []string{"Hello", "World", "Test"},
		}

		resp := sendEmbeddingsRequest(t, payload)
		defer closeBody(resp)

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var embResp core.EmbeddingResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&embResp))

		assert.Equal(t, "list", embResp.Object)
		require.Len(t, embResp.Data, 3)

		for i, d := range embResp.Data {
			assert.Equal(t, "embedding", d.Object)
			assert.Equal(t, i, d.Index)
			assert.NotEmpty(t, d.Embedding)
		}
	})
}

func TestEmbeddingsParameters(t *testing.T) {
	t.Run("encoding_format float", func(t *testing.T) {
		payload := core.EmbeddingRequest{
			Model:          "text-embedding-3-small",
			Input:          "Hello",
			EncodingFormat: "float",
		}

		resp := sendEmbeddingsRequest(t, payload)
		defer closeBody(resp)

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var embResp core.EmbeddingResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&embResp))
		require.Len(t, embResp.Data, 1)
		assert.NotEmpty(t, embResp.Data[0].Embedding)
	})

	t.Run("encoding_format base64", func(t *testing.T) {
		payload := core.EmbeddingRequest{
			Model:          "text-embedding-3-small",
			Input:          "Hello",
			EncodingFormat: "base64",
		}

		resp := sendEmbeddingsRequest(t, payload)
		defer closeBody(resp)

		// base64 format should be accepted (mock returns floats regardless)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("dimensions", func(t *testing.T) {
		dims := 256
		payload := core.EmbeddingRequest{
			Model:      "text-embedding-3-small",
			Input:      "Hello",
			Dimensions: &dims,
		}

		resp := sendEmbeddingsRequest(t, payload)
		defer closeBody(resp)

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var embResp core.EmbeddingResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&embResp))
		require.Len(t, embResp.Data, 1)
		assert.Len(t, embResp.Data[0].Embedding, 256)
	})
}

func TestEmbeddingsUsage(t *testing.T) {
	payload := core.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: "Hello, how are you?",
	}

	resp := sendEmbeddingsRequest(t, payload)
	defer closeBody(resp)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var embResp core.EmbeddingResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&embResp))

	assert.GreaterOrEqual(t, embResp.Usage.PromptTokens, 0)
	assert.Equal(t, embResp.Usage.PromptTokens, embResp.Usage.TotalTokens)
}

func TestEmbeddingsErrors(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		resp, err := http.Post(gatewayURL+embeddingsPath, "application/json",
			strings.NewReader(`{"model": "text-embedding-3-small", "input": invalid}`))
		require.NoError(t, err)
		defer closeBody(resp)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("unsupported model", func(t *testing.T) {
		resp := sendRawEmbeddingsRequest(t, map[string]interface{}{
			"model": "invalid-embedding-model-xyz",
			"input": "Hello",
		})
		defer closeBody(resp)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("missing model", func(t *testing.T) {
		resp := sendRawEmbeddingsRequest(t, map[string]interface{}{
			"input": "Hello",
		})
		defer closeBody(resp)

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestEmbeddingsConcurrency(t *testing.T) {
	const numRequests = 10

	type result struct {
		statusCode int
		err        error
	}
	results := make(chan result, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(idx int) {
			payload := core.EmbeddingRequest{
				Model: "text-embedding-3-small",
				Input: "Concurrent test " + string(rune('A'+idx)),
			}

			resp, err := sendJSONRequestNoT(gatewayURL+embeddingsPath, payload)
			if err != nil {
				results <- result{err: err}
				return
			}
			statusCode := resp.StatusCode
			closeBody(resp)
			results <- result{statusCode: statusCode}
		}(i)
	}

	var errors []error
	successCount := 0
	for i := 0; i < numRequests; i++ {
		select {
		case r := <-results:
			if r.err != nil {
				errors = append(errors, r.err)
			} else if r.statusCode == http.StatusOK {
				successCount++
			}
		case <-time.After(30 * time.Second):
			t.Fatal("Timeout waiting for concurrent requests")
		}
	}

	require.Empty(t, errors, "Expected no request errors")
	assert.Equal(t, numRequests, successCount)
}
