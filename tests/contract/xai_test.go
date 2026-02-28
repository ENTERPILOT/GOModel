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
	"gomodel/internal/providers/xai"
)

func newXAIReplayProvider(t *testing.T, routes map[string]replayRoute) core.Provider {
	t.Helper()

	client := newReplayHTTPClient(t, routes)
	provider := xai.NewWithHTTPClient("xai-test", client, llmclient.Hooks{})
	provider.SetBaseURL("https://replay.local")
	return provider
}

func TestXAIReplayChatCompletion(t *testing.T) {
	testCases := []struct {
		name        string
		fixturePath string
	}{
		{name: "basic", fixturePath: "xai/chat_completion.json"},
		{name: "params", fixturePath: "xai/chat_with_params.json"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := newXAIReplayProvider(t, map[string]replayRoute{
				replayKey(http.MethodPost, "/chat/completions"): jsonFixtureRoute(t, tc.fixturePath),
			})

			resp, err := provider.ChatCompletion(context.Background(), &core.ChatRequest{
				Model: "grok-3-mini",
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
			assert.NotEmpty(t, resp.Choices[0].Message.Content)
		})
	}
}

func TestXAIReplayStreamChatCompletion(t *testing.T) {
	provider := newXAIReplayProvider(t, map[string]replayRoute{
		replayKey(http.MethodPost, "/chat/completions"): sseFixtureRoute(t, "xai/chat_completion_stream.txt"),
	})

	stream, err := provider.StreamChatCompletion(context.Background(), &core.ChatRequest{
		Model: "grok-3-mini",
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

func TestXAIReplayListModels(t *testing.T) {
	provider := newXAIReplayProvider(t, map[string]replayRoute{
		replayKey(http.MethodGet, "/models"): jsonFixtureRoute(t, "xai/models.json"),
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

func TestXAIReplayResponses(t *testing.T) {
	if !goldenFileExists(t, "xai/responses.json") {
		t.Skip("golden file not found - record xai responses first")
	}

	provider := newXAIReplayProvider(t, map[string]replayRoute{
		replayKey(http.MethodPost, "/responses"): jsonFixtureRoute(t, "xai/responses.json"),
	})

	resp, err := provider.Responses(context.Background(), &core.ResponsesRequest{
		Model: "grok-3-mini",
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

func TestXAIReplayStreamResponses(t *testing.T) {
	if !goldenFileExists(t, "xai/responses_stream.txt") {
		t.Skip("golden file not found - record xai responses stream first")
	}

	provider := newXAIReplayProvider(t, map[string]replayRoute{
		replayKey(http.MethodPost, "/responses"): sseFixtureRoute(t, "xai/responses_stream.txt"),
	})

	stream, err := provider.StreamResponses(context.Background(), &core.ResponsesRequest{
		Model: "grok-3-mini",
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
