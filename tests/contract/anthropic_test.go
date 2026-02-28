//go:build contract

package contract

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gomodel/internal/core"
	"gomodel/internal/llmclient"
	"gomodel/internal/providers/anthropic"
)

func newAnthropicReplayProvider(t *testing.T, routes map[string]replayRoute) core.Provider {
	t.Helper()

	client := newReplayHTTPClient(t, routes)
	provider := anthropic.NewWithHTTPClient("sk-ant-test", client, llmclient.Hooks{})
	provider.SetBaseURL("https://replay.local")
	return provider
}

func TestAnthropicReplayChatCompletion(t *testing.T) {
	testCases := []struct {
		name         string
		fixturePath  string
		finishReason string
	}{
		{name: "basic", fixturePath: "anthropic/messages.json"},
		{name: "with-params", fixturePath: "anthropic/messages_with_params.json"},
		{name: "with-tools", fixturePath: "anthropic/messages_with_tools.json", finishReason: "tool_use"},
		{name: "extended-thinking", fixturePath: "anthropic/messages_extended_thinking.json"},
		{name: "multi-turn", fixturePath: "anthropic/messages_multi_turn.json"},
		{name: "multimodal", fixturePath: "anthropic/messages_multimodal.json"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := newAnthropicReplayProvider(t, map[string]replayRoute{
				replayKey(http.MethodPost, "/messages"): jsonFixtureRoute(t, tc.fixturePath),
			})

			resp, err := provider.ChatCompletion(context.Background(), &core.ChatRequest{
				Model: "claude-sonnet-4-20250514",
				Messages: []core.Message{{
					Role:    "user",
					Content: "hello",
				}},
			})
			require.NoError(t, err)
			require.NotNil(t, resp)

			assert.NotEmpty(t, resp.ID)
			assert.Equal(t, "chat.completion", resp.Object)
			require.NotEmpty(t, resp.Choices)
			assert.Equal(t, "assistant", resp.Choices[0].Message.Role)
			assert.NotEmpty(t, resp.Choices[0].FinishReason)
			if tc.finishReason != "" {
				assert.Equal(t, tc.finishReason, resp.Choices[0].FinishReason)
			}
			assert.NotEmpty(t, resp.Choices[0].Message.Content)
		})
	}
}

func TestAnthropicReplayStreamChatCompletion(t *testing.T) {
	provider := newAnthropicReplayProvider(t, map[string]replayRoute{
		replayKey(http.MethodPost, "/messages"): sseFixtureRoute(t, "anthropic/messages_stream.txt"),
	})

	stream, err := provider.StreamChatCompletion(context.Background(), &core.ChatRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []core.Message{{
			Role:    "user",
			Content: "stream",
		}},
	})
	require.NoError(t, err)

	raw := readAllStream(t, stream)
	chunks, done := parseChatStream(t, raw)

	require.True(t, done, "stream should terminate with [DONE]")
	require.NotEmpty(t, chunks)
	assert.NotEmpty(t, extractChatStreamText(chunks))
}

func TestAnthropicReplayResponses(t *testing.T) {
	provider := newAnthropicReplayProvider(t, map[string]replayRoute{
		replayKey(http.MethodPost, "/messages"): jsonFixtureRoute(t, "anthropic/messages.json"),
	})

	resp, err := provider.Responses(context.Background(), &core.ResponsesRequest{
		Model: "claude-sonnet-4-20250514",
		Input: "hello",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "response", resp.Object)
	assert.Equal(t, "completed", resp.Status)
	require.NotEmpty(t, resp.Output)
	require.NotEmpty(t, resp.Output[0].Content)
	assert.NotEmpty(t, resp.Output[0].Content[0].Text)
	require.NotNil(t, resp.Usage)
	assert.GreaterOrEqual(t, resp.Usage.TotalTokens, 0)
}

func TestAnthropicReplayStreamResponses(t *testing.T) {
	provider := newAnthropicReplayProvider(t, map[string]replayRoute{
		replayKey(http.MethodPost, "/messages"): sseFixtureRoute(t, "anthropic/messages_stream.txt"),
	})

	stream, err := provider.StreamResponses(context.Background(), &core.ResponsesRequest{
		Model: "claude-sonnet-4-20250514",
		Input: "stream",
	})
	require.NoError(t, err)

	raw := readAllStream(t, stream)
	events := parseResponsesStream(t, raw)
	require.NotEmpty(t, events)

	assert.True(t, hasResponsesEvent(events, "response.created"))
	assert.True(t, hasResponsesEvent(events, "response.output_text.delta"))
	assert.True(t, hasResponsesEvent(events, "response.completed"))
	assert.NotEmpty(t, extractResponsesStreamText(events))

	hasDone := false
	for _, event := range events {
		if event.Done {
			hasDone = true
			break
		}
	}
	assert.True(t, hasDone, "responses stream should terminate with [DONE]")
}
