//go:build contract

package contract

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gomodel/internal/core"
)

func TestXAI_ChatCompletion(t *testing.T) {
	if !goldenFileExists(t, "xai/chat_completion.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	resp := loadGoldenFile[core.ChatResponse](t, "xai/chat_completion.json")

	t.Run("Contract", func(t *testing.T) {
		// Validate required fields exist (structure validation)
		assert.NotEmpty(t, resp.ID, "response ID should not be empty")
		assert.NotEmpty(t, resp.Object, "response object should not be empty")
		assert.Equal(t, "chat.completion", resp.Object, "object should be chat.completion")
		assert.NotEmpty(t, resp.Model, "model should not be empty")
		assert.NotZero(t, resp.Created, "created timestamp should not be zero")

		// Validate choices structure
		require.NotEmpty(t, resp.Choices, "choices should not be empty")
		choice := resp.Choices[0]
		assert.GreaterOrEqual(t, choice.Index, 0, "choice index should be >= 0")
		assert.NotNil(t, choice.Message, "choice message should not be nil")
		assert.NotEmpty(t, choice.Message.Role, "message role should not be empty")
		assert.Equal(t, "assistant", choice.Message.Role, "message role should be assistant")
		assert.NotEmpty(t, choice.FinishReason, "finish reason should not be empty")

		// Validate usage structure
		assert.GreaterOrEqual(t, resp.Usage.PromptTokens, 0, "prompt tokens should be >= 0")
		assert.GreaterOrEqual(t, resp.Usage.CompletionTokens, 0, "completion tokens should be >= 0")
		assert.GreaterOrEqual(t, resp.Usage.TotalTokens, 0, "total tokens should be >= 0")
	})

	t.Run("ModelPrefix", func(t *testing.T) {
		// xAI models typically contain "grok"
		assert.Contains(t, resp.Model, "grok", "xAI model should contain 'grok'")
	})
}

func TestXAI_ModelsResponse_Contract(t *testing.T) {
	if !goldenFileExists(t, "xai/models.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	resp := loadGoldenFile[core.ModelsResponse](t, "xai/models.json")

	// Validate required fields
	assert.Equal(t, "list", resp.Object, "object should be 'list'")
	assert.NotEmpty(t, resp.Data, "models list should not be empty")

	// Validate each model structure
	for i, model := range resp.Data {
		assert.NotEmpty(t, model.ID, "model %d: ID should not be empty", i)
		assert.Equal(t, "model", model.Object, "model %d: object should be 'model'", i)
		assert.NotEmpty(t, model.OwnedBy, "model %d: owned_by should not be empty", i)
	}

	// Check for some expected models (Grok variants)
	modelIDs := make(map[string]bool)
	for _, model := range resp.Data {
		modelIDs[model.ID] = true
	}

	// At least one Grok model should exist
	hasGrok := false
	for id := range modelIDs {
		if strings.HasPrefix(id, "grok") {
			hasGrok = true
			break
		}
	}
	assert.True(t, hasGrok, "expected at least one Grok model in models list")
}

func TestXAI_StreamingFormat_Contract(t *testing.T) {
	if !goldenFileExists(t, "xai/chat_completion_stream.txt") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	data := loadGoldenFileRaw(t, "xai/chat_completion_stream.txt")

	// Streaming responses should be in SSE format
	assert.Contains(t, string(data), "data:", "streaming response should contain SSE data lines")

	// Should end with [DONE]
	assert.Contains(t, string(data), "[DONE]", "streaming response should end with [DONE]")
}

func TestXAI_Embeddings(t *testing.T) {
	if !goldenFileExists(t, "xai/embeddings.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	resp := loadGoldenFile[core.EmbeddingResponse](t, "xai/embeddings.json")

	t.Run("Contract", func(t *testing.T) {
		assert.Equal(t, "list", resp.Object, "object should be 'list'")
		require.NotEmpty(t, resp.Data, "data should not be empty")

		for i, d := range resp.Data {
			assert.Equal(t, "embedding", d.Object, "data[%d].object should be 'embedding'", i)
			assert.NotEmpty(t, d.Embedding, "data[%d].embedding should not be empty", i)
		}

		assert.GreaterOrEqual(t, resp.Usage.PromptTokens, 0, "prompt_tokens should be >= 0")
		assert.GreaterOrEqual(t, resp.Usage.TotalTokens, 0, "total_tokens should be >= 0")
	})
}

func TestXAI_ChatWithParams(t *testing.T) {
	if !goldenFileExists(t, "xai/chat_with_params.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	resp := loadGoldenFile[core.ChatResponse](t, "xai/chat_with_params.json")

	t.Run("Contract", func(t *testing.T) {
		assert.NotEmpty(t, resp.ID, "response ID should not be empty")
		assert.Equal(t, "chat.completion", resp.Object, "object should be chat.completion")
		require.NotEmpty(t, resp.Choices, "choices should not be empty")
		assert.Equal(t, "assistant", resp.Choices[0].Message.Role, "message role should be assistant")
	})
}
