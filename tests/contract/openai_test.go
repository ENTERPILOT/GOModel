//go:build contract

package contract

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gomodel/internal/core"
)

func TestOpenAI_ChatCompletion(t *testing.T) {
	if !goldenFileExists(t, "openai/chat_completion.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	resp := loadGoldenFile[core.ChatResponse](t, "openai/chat_completion.json")

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

		// Total should equal prompt + completion
		expectedTotal := resp.Usage.PromptTokens + resp.Usage.CompletionTokens
		assert.Equal(t, expectedTotal, resp.Usage.TotalTokens,
			"total tokens should equal prompt + completion")
	})

	t.Run("IDFormat", func(t *testing.T) {
		// OpenAI response IDs typically start with "chatcmpl-"
		assert.Contains(t, resp.ID, "chatcmpl-", "OpenAI chat completion ID should contain 'chatcmpl-'")
	})
}

func TestOpenAI_ModelsResponse_Contract(t *testing.T) {
	if !goldenFileExists(t, "openai/models.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	resp := loadGoldenFile[core.ModelsResponse](t, "openai/models.json")

	// Validate required fields
	assert.Equal(t, "list", resp.Object, "object should be 'list'")
	assert.NotEmpty(t, resp.Data, "models list should not be empty")

	// Validate each model structure
	for i, model := range resp.Data {
		assert.NotEmpty(t, model.ID, "model %d: ID should not be empty", i)
		assert.Equal(t, "model", model.Object, "model %d: object should be 'model'", i)
		assert.NotEmpty(t, model.OwnedBy, "model %d: owned_by should not be empty", i)
	}

	// Check for some expected models
	modelIDs := make(map[string]bool)
	for _, model := range resp.Data {
		modelIDs[model.ID] = true
	}

	// GPT-4 variants should exist (at least one)
	hasGPT4 := modelIDs["gpt-4"] || modelIDs["gpt-4-turbo"] || modelIDs["gpt-4o"] || modelIDs["gpt-4o-mini"]
	assert.True(t, hasGPT4, "expected at least one GPT-4 variant in models list")
}

func TestOpenAI_StreamingFormat_Contract(t *testing.T) {
	if !goldenFileExists(t, "openai/chat_completion_stream.txt") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	data := loadGoldenFileRaw(t, "openai/chat_completion_stream.txt")

	// Streaming responses should be in SSE format
	assert.Contains(t, string(data), "data:", "streaming response should contain SSE data lines")

	// Should end with [DONE]
	assert.Contains(t, string(data), "[DONE]", "streaming response should end with [DONE]")
}

func TestOpenAI_ReasoningModel(t *testing.T) {
	if !goldenFileExists(t, "openai/chat_completion_reasoning.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	resp := loadGoldenFile[core.ChatResponse](t, "openai/chat_completion_reasoning.json")

	t.Run("Contract", func(t *testing.T) {
		// Validate required fields
		assert.NotEmpty(t, resp.ID, "response ID should not be empty")
		assert.Equal(t, "chat.completion", resp.Object, "object should be chat.completion")
		assert.NotEmpty(t, resp.Model, "model should not be empty")
		assert.NotZero(t, resp.Created, "created timestamp should not be zero")

		// Validate choices
		require.NotEmpty(t, resp.Choices, "choices should not be empty")
		choice := resp.Choices[0]
		assert.Equal(t, "assistant", choice.Message.Role, "message role should be assistant")
		assert.NotEmpty(t, choice.FinishReason, "finish reason should not be empty")
	})

	t.Run("ModelType", func(t *testing.T) {
		// Reasoning models contain "o1" or "o3" in their name
		isReasoningModel := false
		if len(resp.Model) >= 2 {
			prefix := resp.Model[:2]
			if prefix == "o1" || prefix == "o3" {
				isReasoningModel = true
			}
		}
		assert.True(t, isReasoningModel, "model should be a reasoning model (o1 or o3 series)")
	})
}

func TestOpenAI_ChatWithTools(t *testing.T) {
	if !goldenFileExists(t, "openai/chat_with_tools.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	resp := loadGoldenFile[ToolCallResponse](t, "openai/chat_with_tools.json")

	t.Run("Contract", func(t *testing.T) {
		assert.NotEmpty(t, resp.ID, "response ID should not be empty")
		assert.Equal(t, "chat.completion", resp.Object, "object should be chat.completion")
		require.NotEmpty(t, resp.Choices, "choices should not be empty")

		choice := resp.Choices[0]
		// Tool calls should be present when model uses function calling
		if len(choice.Message.ToolCalls) > 0 {
			for i, tc := range choice.Message.ToolCalls {
				assert.NotEmpty(t, tc.ID, "tool call %d: ID should not be empty", i)
				assert.Equal(t, "function", tc.Type, "tool call %d: type should be 'function'", i)
				assert.NotEmpty(t, tc.Function.Name, "tool call %d: function name should not be empty", i)
			}
			assert.Equal(t, "tool_calls", choice.FinishReason, "finish reason should be 'tool_calls' when tools are called")
		}
	})
}

func TestOpenAI_ChatWithParams(t *testing.T) {
	if !goldenFileExists(t, "openai/chat_with_params.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	resp := loadGoldenFile[core.ChatResponse](t, "openai/chat_with_params.json")

	t.Run("Contract", func(t *testing.T) {
		assert.NotEmpty(t, resp.ID, "response ID should not be empty")
		assert.Equal(t, "chat.completion", resp.Object, "object should be chat.completion")
		require.NotEmpty(t, resp.Choices, "choices should not be empty")
		assert.Equal(t, "assistant", resp.Choices[0].Message.Role, "message role should be assistant")

		// When stop sequence is hit, finish_reason should be "stop"
		assert.Equal(t, "stop", resp.Choices[0].FinishReason, "finish reason should be 'stop'")
	})
}

func TestOpenAI_ChatMultiTurn(t *testing.T) {
	if !goldenFileExists(t, "openai/chat_multi_turn.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	resp := loadGoldenFile[core.ChatResponse](t, "openai/chat_multi_turn.json")

	t.Run("Contract", func(t *testing.T) {
		assert.NotEmpty(t, resp.ID, "response ID should not be empty")
		assert.Equal(t, "chat.completion", resp.Object, "object should be chat.completion")
		require.NotEmpty(t, resp.Choices, "choices should not be empty")
		assert.Equal(t, "assistant", resp.Choices[0].Message.Role, "message role should be assistant")
		assert.NotEmpty(t, resp.Choices[0].Message.Content, "message content should not be empty")
	})
}

func TestOpenAI_ChatMultimodal(t *testing.T) {
	if !goldenFileExists(t, "openai/chat_multimodal.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	resp := loadGoldenFile[core.ChatResponse](t, "openai/chat_multimodal.json")

	t.Run("Contract", func(t *testing.T) {
		assert.NotEmpty(t, resp.ID, "response ID should not be empty")
		assert.Equal(t, "chat.completion", resp.Object, "object should be chat.completion")
		require.NotEmpty(t, resp.Choices, "choices should not be empty")
		assert.Equal(t, "assistant", resp.Choices[0].Message.Role, "message role should be assistant")
		assert.NotEmpty(t, resp.Choices[0].Message.Content, "message content should not be empty")
	})
}

func TestOpenAI_JSONMode(t *testing.T) {
	if !goldenFileExists(t, "openai/chat_json_mode.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	resp := loadGoldenFile[core.ChatResponse](t, "openai/chat_json_mode.json")

	t.Run("Contract", func(t *testing.T) {
		assert.NotEmpty(t, resp.ID, "response ID should not be empty")
		assert.Equal(t, "chat.completion", resp.Object, "object should be chat.completion")
		require.NotEmpty(t, resp.Choices, "choices should not be empty")
		assert.Equal(t, "assistant", resp.Choices[0].Message.Role, "message role should be assistant")
		assert.NotEmpty(t, resp.Choices[0].Message.Content, "message content should not be empty")
	})
}
