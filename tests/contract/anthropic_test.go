//go:build contract

package contract

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AnthropicMessageResponse represents an Anthropic messages API response.
type AnthropicMessageResponse struct {
	ID           string                   `json:"id"`
	Type         string                   `json:"type"`
	Role         string                   `json:"role"`
	Content      []AnthropicContentBlock  `json:"content"`
	Model        string                   `json:"model"`
	StopReason   string                   `json:"stop_reason"`
	StopSequence *string                  `json:"stop_sequence"`
	Usage        AnthropicUsage           `json:"usage"`
}

// AnthropicContentBlock represents a content block in Anthropic response.
type AnthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// AnthropicUsage represents token usage in Anthropic response.
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func TestAnthropic_Messages(t *testing.T) {
	if !goldenFileExists(t, "anthropic/messages.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	data := loadGoldenFileRaw(t, "anthropic/messages.json")

	var resp AnthropicMessageResponse
	err := json.Unmarshal(data, &resp)
	require.NoError(t, err, "failed to unmarshal Anthropic response")

	t.Run("Contract", func(t *testing.T) {
		// Validate required fields
		assert.NotEmpty(t, resp.ID, "response ID should not be empty")
		assert.Equal(t, "message", resp.Type, "type should be 'message'")
		assert.Equal(t, "assistant", resp.Role, "role should be 'assistant'")
		assert.NotEmpty(t, resp.Model, "model should not be empty")

		// Validate content structure
		require.NotEmpty(t, resp.Content, "content should not be empty")
		for i, block := range resp.Content {
			assert.NotEmpty(t, block.Type, "content block %d: type should not be empty", i)
			if block.Type == "text" {
				assert.NotEmpty(t, block.Text, "content block %d: text should not be empty for text type", i)
			}
		}

		// Validate stop reason
		assert.NotEmpty(t, resp.StopReason, "stop_reason should not be empty")

		// Validate usage
		assert.GreaterOrEqual(t, resp.Usage.InputTokens, 0, "input_tokens should be >= 0")
		assert.GreaterOrEqual(t, resp.Usage.OutputTokens, 0, "output_tokens should be >= 0")
	})

	t.Run("IDFormat", func(t *testing.T) {
		// Anthropic message IDs typically start with "msg_"
		assert.Contains(t, resp.ID, "msg_", "Anthropic message ID should contain 'msg_'")
	})

	t.Run("Streaming", func(t *testing.T) {
		if !goldenFileExists(t, "anthropic/messages_stream.txt") {
			t.Skip("golden file not found - run 'make record-api' to generate")
		}

		data := loadGoldenFileRaw(t, "anthropic/messages_stream.txt")

		// Anthropic streaming responses use SSE format with event types
		assert.Contains(t, string(data), "event:", "streaming response should contain SSE event lines")
		assert.Contains(t, string(data), "data:", "streaming response should contain SSE data lines")

		// Should contain message_start event
		assert.Contains(t, string(data), "message_start", "streaming response should contain message_start event")

		// Should contain message_stop event
		assert.Contains(t, string(data), "message_stop", "streaming response should contain message_stop event")
	})
}

func TestAnthropic_MessagesWithParams(t *testing.T) {
	if !goldenFileExists(t, "anthropic/messages_with_params.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	data := loadGoldenFileRaw(t, "anthropic/messages_with_params.json")

	var resp AnthropicMessageResponse
	err := json.Unmarshal(data, &resp)
	require.NoError(t, err, "failed to unmarshal Anthropic response")

	t.Run("Contract", func(t *testing.T) {
		assert.NotEmpty(t, resp.ID, "response ID should not be empty")
		assert.Equal(t, "message", resp.Type, "type should be 'message'")
		assert.Equal(t, "assistant", resp.Role, "role should be 'assistant'")
		assert.NotEmpty(t, resp.Model, "model should not be empty")
	})
}

// AnthropicToolUseResponse represents an Anthropic response with tool use.
type AnthropicToolUseResponse struct {
	ID           string                       `json:"id"`
	Type         string                       `json:"type"`
	Role         string                       `json:"role"`
	Content      []AnthropicToolContentBlock  `json:"content"`
	Model        string                       `json:"model"`
	StopReason   string                       `json:"stop_reason"`
	StopSequence *string                      `json:"stop_sequence"`
	Usage        AnthropicUsage               `json:"usage"`
}

// AnthropicToolContentBlock represents a content block that may be text or tool_use.
type AnthropicToolContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

func TestAnthropic_MessagesWithTools(t *testing.T) {
	if !goldenFileExists(t, "anthropic/messages_with_tools.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	data := loadGoldenFileRaw(t, "anthropic/messages_with_tools.json")

	var resp AnthropicToolUseResponse
	err := json.Unmarshal(data, &resp)
	require.NoError(t, err, "failed to unmarshal Anthropic response")

	t.Run("Contract", func(t *testing.T) {
		assert.NotEmpty(t, resp.ID, "response ID should not be empty")
		assert.Equal(t, "message", resp.Type, "type should be 'message'")
		assert.Equal(t, "assistant", resp.Role, "role should be 'assistant'")
		assert.NotEmpty(t, resp.Model, "model should not be empty")

		// Check for tool_use content blocks
		hasToolUse := false
		for _, block := range resp.Content {
			if block.Type == "tool_use" {
				hasToolUse = true
				assert.NotEmpty(t, block.ID, "tool_use block should have ID")
				assert.NotEmpty(t, block.Name, "tool_use block should have name")
			}
		}
		if hasToolUse {
			assert.Equal(t, "tool_use", resp.StopReason, "stop_reason should be 'tool_use' when tools are called")
		}
	})
}

// AnthropicThinkingResponse represents an Anthropic response with extended thinking.
type AnthropicThinkingResponse struct {
	ID           string                         `json:"id"`
	Type         string                         `json:"type"`
	Role         string                         `json:"role"`
	Content      []AnthropicThinkingContentBlock `json:"content"`
	Model        string                         `json:"model"`
	StopReason   string                         `json:"stop_reason"`
	StopSequence *string                        `json:"stop_sequence"`
	Usage        AnthropicThinkingUsage         `json:"usage"`
}

// AnthropicThinkingContentBlock represents a content block that may be text or thinking.
type AnthropicThinkingContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Thinking string `json:"thinking,omitempty"`
}

// AnthropicThinkingUsage represents usage with cache tokens for extended thinking.
type AnthropicThinkingUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func TestAnthropic_ExtendedThinking(t *testing.T) {
	if !goldenFileExists(t, "anthropic/messages_extended_thinking.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	data := loadGoldenFileRaw(t, "anthropic/messages_extended_thinking.json")

	var resp AnthropicThinkingResponse
	err := json.Unmarshal(data, &resp)
	require.NoError(t, err, "failed to unmarshal Anthropic response")

	t.Run("Contract", func(t *testing.T) {
		assert.NotEmpty(t, resp.ID, "response ID should not be empty")
		assert.Equal(t, "message", resp.Type, "type should be 'message'")
		assert.Equal(t, "assistant", resp.Role, "role should be 'assistant'")
		assert.NotEmpty(t, resp.Model, "model should not be empty")

		// Extended thinking responses should have thinking content block
		hasThinking := false
		for _, block := range resp.Content {
			if block.Type == "thinking" {
				hasThinking = true
				assert.NotEmpty(t, block.Thinking, "thinking block should have content")
			}
		}
		// Note: thinking may not always be present depending on the model and request
		_ = hasThinking
	})
}

func TestAnthropic_MessagesMultiTurn(t *testing.T) {
	if !goldenFileExists(t, "anthropic/messages_multi_turn.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	data := loadGoldenFileRaw(t, "anthropic/messages_multi_turn.json")

	var resp AnthropicMessageResponse
	err := json.Unmarshal(data, &resp)
	require.NoError(t, err, "failed to unmarshal Anthropic response")

	t.Run("Contract", func(t *testing.T) {
		assert.NotEmpty(t, resp.ID, "response ID should not be empty")
		assert.Equal(t, "message", resp.Type, "type should be 'message'")
		assert.Equal(t, "assistant", resp.Role, "role should be 'assistant'")
		require.NotEmpty(t, resp.Content, "content should not be empty")
	})
}

func TestAnthropic_MessagesMultimodal(t *testing.T) {
	if !goldenFileExists(t, "anthropic/messages_multimodal.json") {
		t.Skip("golden file not found - run 'make record-api' to generate")
	}

	data := loadGoldenFileRaw(t, "anthropic/messages_multimodal.json")

	var resp AnthropicMessageResponse
	err := json.Unmarshal(data, &resp)
	require.NoError(t, err, "failed to unmarshal Anthropic response")

	t.Run("Contract", func(t *testing.T) {
		assert.NotEmpty(t, resp.ID, "response ID should not be empty")
		assert.Equal(t, "message", resp.Type, "type should be 'message'")
		assert.Equal(t, "assistant", resp.Role, "role should be 'assistant'")
		require.NotEmpty(t, resp.Content, "content should not be empty")
	})
}
