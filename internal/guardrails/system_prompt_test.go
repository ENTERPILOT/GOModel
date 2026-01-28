package guardrails

import (
	"testing"

	"gomodel/internal/core"
)

func TestSystemPromptInjector_ProcessChatRequest(t *testing.T) {
	tests := []struct {
		name           string
		config         SystemPromptConfig
		request        *core.ChatRequest
		providerType   string
		wantMessages   []core.Message
		wantUnchanged  bool
	}{
		{
			name: "no rules configured",
			config: SystemPromptConfig{
				Enabled: true,
			},
			request: &core.ChatRequest{
				Model: "gpt-4",
				Messages: []core.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			providerType:  "openai",
			wantUnchanged: true,
		},
		{
			name: "global prepend without existing system",
			config: SystemPromptConfig{
				Enabled: true,
				Global: &SystemPromptRule{
					Prompt:   "You are helpful.",
					Position: PositionPrepend,
				},
			},
			request: &core.ChatRequest{
				Model: "gpt-4",
				Messages: []core.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			providerType: "openai",
			wantMessages: []core.Message{
				{Role: "system", Content: "You are helpful."},
				{Role: "user", Content: "Hello"},
			},
		},
		{
			name: "global prepend with existing system - preserve",
			config: SystemPromptConfig{
				Enabled: true,
				Global: &SystemPromptRule{
					Prompt:             "You are helpful.",
					Position:           PositionPrepend,
					PreserveUserSystem: true,
				},
			},
			request: &core.ChatRequest{
				Model: "gpt-4",
				Messages: []core.Message{
					{Role: "system", Content: "Be concise."},
					{Role: "user", Content: "Hello"},
				},
			},
			providerType: "openai",
			wantMessages: []core.Message{
				{Role: "system", Content: "You are helpful.\n\nBe concise."},
				{Role: "user", Content: "Hello"},
			},
		},
		{
			name: "global prepend with existing system - no preserve",
			config: SystemPromptConfig{
				Enabled: true,
				Global: &SystemPromptRule{
					Prompt:             "You are helpful.",
					Position:           PositionPrepend,
					PreserveUserSystem: false,
				},
			},
			request: &core.ChatRequest{
				Model: "gpt-4",
				Messages: []core.Message{
					{Role: "system", Content: "Be concise."},
					{Role: "user", Content: "Hello"},
				},
			},
			providerType: "openai",
			wantMessages: []core.Message{
				{Role: "system", Content: "You are helpful."},
				{Role: "user", Content: "Hello"},
			},
		},
		{
			name: "append with existing system - preserve",
			config: SystemPromptConfig{
				Enabled: true,
				Global: &SystemPromptRule{
					Prompt:             "Always be polite.",
					Position:           PositionAppend,
					PreserveUserSystem: true,
				},
			},
			request: &core.ChatRequest{
				Model: "gpt-4",
				Messages: []core.Message{
					{Role: "system", Content: "Be concise."},
					{Role: "user", Content: "Hello"},
				},
			},
			providerType: "openai",
			wantMessages: []core.Message{
				{Role: "system", Content: "Be concise.\n\nAlways be polite."},
				{Role: "user", Content: "Hello"},
			},
		},
		{
			name: "replace position",
			config: SystemPromptConfig{
				Enabled: true,
				Global: &SystemPromptRule{
					Prompt:   "New system prompt.",
					Position: PositionReplace,
				},
			},
			request: &core.ChatRequest{
				Model: "gpt-4",
				Messages: []core.Message{
					{Role: "system", Content: "Old prompt."},
					{Role: "user", Content: "Hello"},
				},
			},
			providerType: "openai",
			wantMessages: []core.Message{
				{Role: "system", Content: "New system prompt."},
				{Role: "user", Content: "Hello"},
			},
		},
		{
			name: "model-specific takes precedence over global",
			config: SystemPromptConfig{
				Enabled: true,
				Global: &SystemPromptRule{
					Prompt:   "Global prompt.",
					Position: PositionPrepend,
				},
				Models: map[string]SystemPromptRule{
					"gpt-4": {
						Prompt:   "GPT-4 specific.",
						Position: PositionPrepend,
					},
				},
			},
			request: &core.ChatRequest{
				Model: "gpt-4",
				Messages: []core.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			providerType: "openai",
			wantMessages: []core.Message{
				{Role: "system", Content: "GPT-4 specific."},
				{Role: "user", Content: "Hello"},
			},
		},
		{
			name: "provider-specific takes precedence over global",
			config: SystemPromptConfig{
				Enabled: true,
				Global: &SystemPromptRule{
					Prompt:   "Global prompt.",
					Position: PositionPrepend,
				},
				Providers: map[string]SystemPromptRule{
					"anthropic": {
						Prompt:   "Anthropic specific.",
						Position: PositionPrepend,
					},
				},
			},
			request: &core.ChatRequest{
				Model: "claude-3-opus",
				Messages: []core.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			providerType: "anthropic",
			wantMessages: []core.Message{
				{Role: "system", Content: "Anthropic specific."},
				{Role: "user", Content: "Hello"},
			},
		},
		{
			name: "model-specific takes precedence over provider",
			config: SystemPromptConfig{
				Enabled: true,
				Providers: map[string]SystemPromptRule{
					"openai": {
						Prompt:   "OpenAI default.",
						Position: PositionPrepend,
					},
				},
				Models: map[string]SystemPromptRule{
					"gpt-4": {
						Prompt:   "GPT-4 override.",
						Position: PositionPrepend,
					},
				},
			},
			request: &core.ChatRequest{
				Model: "gpt-4",
				Messages: []core.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			providerType: "openai",
			wantMessages: []core.Message{
				{Role: "system", Content: "GPT-4 override."},
				{Role: "user", Content: "Hello"},
			},
		},
		{
			name: "default position is prepend",
			config: SystemPromptConfig{
				Enabled: true,
				Global: &SystemPromptRule{
					Prompt: "Default position test.",
				},
			},
			request: &core.ChatRequest{
				Model: "gpt-4",
				Messages: []core.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			providerType: "openai",
			wantMessages: []core.Message{
				{Role: "system", Content: "Default position test."},
				{Role: "user", Content: "Hello"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			injector := NewSystemPromptInjector(tt.config)
			result := injector.ProcessChatRequest(tt.request, tt.providerType)

			if tt.wantUnchanged {
				if result != tt.request {
					t.Error("expected unchanged request pointer")
				}
				return
			}

			if len(result.Messages) != len(tt.wantMessages) {
				t.Errorf("message count: got %d, want %d", len(result.Messages), len(tt.wantMessages))
				return
			}

			for i, want := range tt.wantMessages {
				got := result.Messages[i]
				if got.Role != want.Role {
					t.Errorf("message[%d] role: got %q, want %q", i, got.Role, want.Role)
				}
				if got.Content != want.Content {
					t.Errorf("message[%d] content: got %q, want %q", i, got.Content, want.Content)
				}
			}
		})
	}
}

func TestSystemPromptInjector_ProcessResponsesRequest(t *testing.T) {
	tests := []struct {
		name             string
		config           SystemPromptConfig
		request          *core.ResponsesRequest
		providerType     string
		wantInstructions string
		wantUnchanged    bool
	}{
		{
			name: "prepend to empty instructions",
			config: SystemPromptConfig{
				Enabled: true,
				Global: &SystemPromptRule{
					Prompt:   "You are helpful.",
					Position: PositionPrepend,
				},
			},
			request: &core.ResponsesRequest{
				Model:        "gpt-4",
				Instructions: "",
			},
			providerType:     "openai",
			wantInstructions: "You are helpful.",
		},
		{
			name: "prepend to existing instructions - preserve",
			config: SystemPromptConfig{
				Enabled: true,
				Global: &SystemPromptRule{
					Prompt:             "You are helpful.",
					Position:           PositionPrepend,
					PreserveUserSystem: true,
				},
			},
			request: &core.ResponsesRequest{
				Model:        "gpt-4",
				Instructions: "Be concise.",
			},
			providerType:     "openai",
			wantInstructions: "You are helpful.\n\nBe concise.",
		},
		{
			name: "append to existing instructions - preserve",
			config: SystemPromptConfig{
				Enabled: true,
				Global: &SystemPromptRule{
					Prompt:             "Always be polite.",
					Position:           PositionAppend,
					PreserveUserSystem: true,
				},
			},
			request: &core.ResponsesRequest{
				Model:        "gpt-4",
				Instructions: "Be concise.",
			},
			providerType:     "openai",
			wantInstructions: "Be concise.\n\nAlways be polite.",
		},
		{
			name: "replace instructions",
			config: SystemPromptConfig{
				Enabled: true,
				Global: &SystemPromptRule{
					Prompt:   "New instructions.",
					Position: PositionReplace,
				},
			},
			request: &core.ResponsesRequest{
				Model:        "gpt-4",
				Instructions: "Old instructions.",
			},
			providerType:     "openai",
			wantInstructions: "New instructions.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			injector := NewSystemPromptInjector(tt.config)
			result := injector.ProcessResponsesRequest(tt.request, tt.providerType)

			if tt.wantUnchanged {
				if result != tt.request {
					t.Error("expected unchanged request pointer")
				}
				return
			}

			if result.Instructions != tt.wantInstructions {
				t.Errorf("instructions: got %q, want %q", result.Instructions, tt.wantInstructions)
			}
		})
	}
}
