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
	"gomodel/internal/providers/openai"
)

func newOpenAIReplayProvider(t *testing.T, routes map[string]replayRoute) core.Provider {
	t.Helper()

	client := newReplayHTTPClient(t, routes)
	provider := openai.NewWithHTTPClient("sk-test", client, llmclient.Hooks{})
	provider.SetBaseURL("https://replay.local")
	return provider
}

func TestOpenAIReplayChatCompletion(t *testing.T) {
	testCases := []struct {
		name          string
		fixturePath   string
		expectContent bool
		finishReason  string
	}{
		{name: "basic", fixturePath: "openai/chat_completion.json", expectContent: true, finishReason: "stop"},
		{name: "reasoning", fixturePath: "openai/chat_completion_reasoning.json", expectContent: true},
		{name: "json-mode", fixturePath: "openai/chat_json_mode.json", expectContent: true},
		{name: "params", fixturePath: "openai/chat_with_params.json", expectContent: true, finishReason: "stop"},
		{name: "multi-turn", fixturePath: "openai/chat_multi_turn.json", expectContent: true},
		{name: "multimodal", fixturePath: "openai/chat_multimodal.json", expectContent: true},
		{name: "tools", fixturePath: "openai/chat_with_tools.json", expectContent: false, finishReason: "tool_calls"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := newOpenAIReplayProvider(t, map[string]replayRoute{
				replayKey(http.MethodPost, "/chat/completions"): jsonFixtureRoute(t, tc.fixturePath),
			})

			resp, err := provider.ChatCompletion(context.Background(), &core.ChatRequest{
				Model: "gpt-4o-mini",
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
			if tc.expectContent {
				assert.NotEmpty(t, resp.Choices[0].Message.Content)
			}
		})
	}
}

func TestOpenAIReplayStreamChatCompletion(t *testing.T) {
	provider := newOpenAIReplayProvider(t, map[string]replayRoute{
		replayKey(http.MethodPost, "/chat/completions"): sseFixtureRoute(t, "openai/chat_completion_stream.txt"),
	})

	stream, err := provider.StreamChatCompletion(context.Background(), &core.ChatRequest{
		Model: "gpt-4o-mini",
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

func TestOpenAIReplayListModels(t *testing.T) {
	provider := newOpenAIReplayProvider(t, map[string]replayRoute{
		replayKey(http.MethodGet, "/models"): jsonFixtureRoute(t, "openai/models.json"),
	})

	resp, err := provider.ListModels(context.Background())
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "list", resp.Object)
	require.NotEmpty(t, resp.Data)
	for _, model := range resp.Data {
		assert.NotEmpty(t, model.ID)
		assert.Equal(t, "model", model.Object)
	}
}

func TestOpenAIReplayResponses(t *testing.T) {
	if !goldenFileExists(t, "openai/responses.json") {
		t.Skip("golden file not found - record openai responses first")
	}

	provider := newOpenAIReplayProvider(t, map[string]replayRoute{
		replayKey(http.MethodPost, "/responses"): jsonFixtureRoute(t, "openai/responses.json"),
	})

	resp, err := provider.Responses(context.Background(), &core.ResponsesRequest{
		Model: "gpt-4o-mini",
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

func TestOpenAIReplayStreamResponses(t *testing.T) {
	if !goldenFileExists(t, "openai/responses_stream.txt") {
		t.Skip("golden file not found - record openai responses stream first")
	}

	provider := newOpenAIReplayProvider(t, map[string]replayRoute{
		replayKey(http.MethodPost, "/responses"): sseFixtureRoute(t, "openai/responses_stream.txt"),
	})

	stream, err := provider.StreamResponses(context.Background(), &core.ResponsesRequest{
		Model: "gpt-4o-mini",
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
}
