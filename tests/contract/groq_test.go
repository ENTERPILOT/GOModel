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
	"gomodel/internal/providers/groq"
)

func newGroqReplayProvider(t *testing.T, routes map[string]replayRoute) core.Provider {
	t.Helper()

	client := newReplayHTTPClient(t, routes)
	provider := groq.NewWithHTTPClient("gsk-test", client, llmclient.Hooks{})
	provider.SetBaseURL("https://replay.local")
	return provider
}

func TestGroqReplayChatCompletion(t *testing.T) {
	testCases := []struct {
		name          string
		fixturePath   string
		expectContent bool
	}{
		{name: "basic", fixturePath: "groq/chat_completion.json", expectContent: true},
		{name: "params", fixturePath: "groq/chat_with_params.json", expectContent: true},
		{name: "tools", fixturePath: "groq/chat_with_tools.json", expectContent: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := newGroqReplayProvider(t, map[string]replayRoute{
				replayKey(http.MethodPost, "/chat/completions"): jsonFixtureRoute(t, tc.fixturePath),
			})

			resp, err := provider.ChatCompletion(context.Background(), &core.ChatRequest{
				Model: "llama-3.3-70b-versatile",
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
			if tc.expectContent {
				assert.NotEmpty(t, resp.Choices[0].Message.Content)
			}
		})
	}
}

func TestGroqReplayStreamChatCompletion(t *testing.T) {
	provider := newGroqReplayProvider(t, map[string]replayRoute{
		replayKey(http.MethodPost, "/chat/completions"): sseFixtureRoute(t, "groq/chat_completion_stream.txt"),
	})

	stream, err := provider.StreamChatCompletion(context.Background(), &core.ChatRequest{
		Model: "llama-3.3-70b-versatile",
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

func TestGroqReplayListModels(t *testing.T) {
	provider := newGroqReplayProvider(t, map[string]replayRoute{
		replayKey(http.MethodGet, "/models"): jsonFixtureRoute(t, "groq/models.json"),
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

func TestGroqReplayResponses(t *testing.T) {
	provider := newGroqReplayProvider(t, map[string]replayRoute{
		replayKey(http.MethodPost, "/chat/completions"): jsonFixtureRoute(t, "groq/chat_completion.json"),
	})

	resp, err := provider.Responses(context.Background(), &core.ResponsesRequest{
		Model: "llama-3.3-70b-versatile",
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

func TestGroqReplayStreamResponses(t *testing.T) {
	provider := newGroqReplayProvider(t, map[string]replayRoute{
		replayKey(http.MethodPost, "/chat/completions"): sseFixtureRoute(t, "groq/chat_completion_stream.txt"),
	})

	stream, err := provider.StreamResponses(context.Background(), &core.ResponsesRequest{
		Model: "llama-3.3-70b-versatile",
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
