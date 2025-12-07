package anthropic

import (
	"testing"

	"gomodel/internal/core"
)

func TestSupports(t *testing.T) {
	provider := New("test-api-key")

	tests := []struct {
		model    string
		expected bool
	}{
		{"claude-3-5-sonnet-20241022", true},
		{"claude-3-opus-20240229", true},
		{"claude-3-haiku-20240307", true},
		{"gpt-4o", false},
		{"gpt-4o-mini", false},
		{"o1-preview", false},
		{"random-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := provider.Supports(tt.model)
			if result != tt.expected {
				t.Errorf("Supports(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestConvertToAnthropicRequest(t *testing.T) {
	temp := 0.7
	maxTokens := 1024

	tests := []struct {
		name     string
		input    *core.ChatRequest
		checkFn  func(*testing.T, *anthropicRequest)
	}{
		{
			name: "basic request",
			input: &core.ChatRequest{
				Model: "claude-3-5-sonnet-20241022",
				Messages: []core.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			checkFn: func(t *testing.T, req *anthropicRequest) {
				if req.Model != "claude-3-5-sonnet-20241022" {
					t.Errorf("Model = %q, want %q", req.Model, "claude-3-5-sonnet-20241022")
				}
				if len(req.Messages) != 1 {
					t.Errorf("len(Messages) = %d, want 1", len(req.Messages))
				}
				if req.Messages[0].Content != "Hello" {
					t.Errorf("Message content = %q, want %q", req.Messages[0].Content, "Hello")
				}
				if req.MaxTokens != 4096 {
					t.Errorf("MaxTokens = %d, want 4096", req.MaxTokens)
				}
			},
		},
		{
			name: "request with system message",
			input: &core.ChatRequest{
				Model: "claude-3-opus-20240229",
				Messages: []core.Message{
					{Role: "system", Content: "You are a helpful assistant"},
					{Role: "user", Content: "Hello"},
				},
			},
			checkFn: func(t *testing.T, req *anthropicRequest) {
				if req.System != "You are a helpful assistant" {
					t.Errorf("System = %q, want %q", req.System, "You are a helpful assistant")
				}
				if len(req.Messages) != 1 {
					t.Errorf("len(Messages) = %d, want 1 (system should be extracted)", len(req.Messages))
				}
			},
		},
		{
			name: "request with parameters",
			input: &core.ChatRequest{
				Model:       "claude-3-5-sonnet-20241022",
				Temperature: &temp,
				MaxTokens:   &maxTokens,
				Messages: []core.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			checkFn: func(t *testing.T, req *anthropicRequest) {
				if req.Temperature == nil || *req.Temperature != 0.7 {
					t.Errorf("Temperature = %v, want 0.7", req.Temperature)
				}
				if req.MaxTokens != 1024 {
					t.Errorf("MaxTokens = %d, want 1024", req.MaxTokens)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToAnthropicRequest(tt.input)
			tt.checkFn(t, result)
		})
	}
}

func TestConvertFromAnthropicResponse(t *testing.T) {
	resp := &anthropicResponse{
		ID:    "msg_123",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-3-5-sonnet-20241022",
		Content: []anthropicContent{
			{Type: "text", Text: "Hello! How can I help you today?"},
		},
		StopReason: "end_turn",
		Usage: anthropicUsage{
			InputTokens:  10,
			OutputTokens: 20,
		},
	}

	result := convertFromAnthropicResponse(resp)

	if result.ID != "msg_123" {
		t.Errorf("ID = %q, want %q", result.ID, "msg_123")
	}
	if result.Object != "chat.completion" {
		t.Errorf("Object = %q, want %q", result.Object, "chat.completion")
	}
	if result.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("Model = %q, want %q", result.Model, "claude-3-5-sonnet-20241022")
	}
	if len(result.Choices) != 1 {
		t.Fatalf("len(Choices) = %d, want 1", len(result.Choices))
	}
	if result.Choices[0].Message.Content != "Hello! How can I help you today?" {
		t.Errorf("Message content = %q, want %q", result.Choices[0].Message.Content, "Hello! How can I help you today?")
	}
	if result.Choices[0].Message.Role != "assistant" {
		t.Errorf("Message role = %q, want %q", result.Choices[0].Message.Role, "assistant")
	}
	if result.Choices[0].FinishReason != "end_turn" {
		t.Errorf("FinishReason = %q, want %q", result.Choices[0].FinishReason, "end_turn")
	}
	if result.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d, want 10", result.Usage.PromptTokens)
	}
	if result.Usage.CompletionTokens != 20 {
		t.Errorf("CompletionTokens = %d, want 20", result.Usage.CompletionTokens)
	}
	if result.Usage.TotalTokens != 30 {
		t.Errorf("TotalTokens = %d, want 30", result.Usage.TotalTokens)
	}
}

