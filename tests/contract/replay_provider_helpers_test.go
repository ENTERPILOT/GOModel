//go:build contract

package contract

import (
	"testing"

	"gomodel/internal/core"
	"gomodel/internal/llmclient"
	"gomodel/internal/providers/gemini"
)

func newGeminiReplayProvider(t *testing.T, routes map[string]replayRoute) core.Provider {
	t.Helper()

	client := newReplayHTTPClient(t, routes)
	provider := gemini.NewWithHTTPClient("test-api-key", client, llmclient.Hooks{})
	provider.SetBaseURL("https://replay.local")
	provider.SetModelsURL("https://replay.local")
	return provider
}
