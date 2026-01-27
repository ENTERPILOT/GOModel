package guardrails

import (
	"io"
	"strings"
	"testing"

	"gomodel/internal/core"
)

func TestProcessor_Nil(t *testing.T) {
	var p *Processor

	// Should handle nil processor gracefully
	req := &core.ChatRequest{
		Model:    "gpt-4",
		Messages: []core.Message{{Role: "user", Content: "Hello"}},
	}

	result, ctx := p.ProcessChatRequest(req, "openai")
	if result != req {
		t.Error("nil processor should return original request")
	}
	if ctx != nil {
		t.Error("nil processor should return nil context")
	}
}

func TestProcessor_SystemPromptOnly(t *testing.T) {
	cfg := Config{
		SystemPrompt: SystemPromptConfig{
			Enabled: true,
			Global: &SystemPromptRule{
				Prompt:   "You are helpful.",
				Position: PositionPrepend,
			},
		},
		Anonymization: AnonymizationConfig{
			Enabled: false,
		},
	}

	p := New(cfg)

	req := &core.ChatRequest{
		Model: "gpt-4",
		Messages: []core.Message{
			{Role: "user", Content: "Hello test@example.com"},
		},
	}

	result, ctx := p.ProcessChatRequest(req, "openai")

	// Should have system prompt injected
	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.Messages))
	}
	if result.Messages[0].Content != "You are helpful." {
		t.Error("system prompt not injected correctly")
	}

	// Email should NOT be anonymized (anonymization disabled)
	if !strings.Contains(result.Messages[1].Content, "test@example.com") {
		t.Error("email should not be anonymized when anonymization is disabled")
	}

	// Context should have empty anonymization map
	if len(ctx.AnonymizationMap) != 0 {
		t.Error("anonymization map should be empty")
	}
}

func TestProcessor_AnonymizationOnly(t *testing.T) {
	cfg := Config{
		SystemPrompt: SystemPromptConfig{
			Enabled: false,
		},
		Anonymization: AnonymizationConfig{
			Enabled:              true,
			Strategy:             StrategyToken,
			DeanonymizeResponses: true,
			Detectors: DetectorConfig{
				Email: true,
			},
		},
	}

	p := New(cfg)

	req := &core.ChatRequest{
		Model: "gpt-4",
		Messages: []core.Message{
			{Role: "user", Content: "Contact me at test@example.com"},
		},
	}

	result, ctx := p.ProcessChatRequest(req, "openai")

	// Email should be anonymized
	if strings.Contains(result.Messages[0].Content, "test@example.com") {
		t.Error("email should be anonymized")
	}
	if !strings.Contains(result.Messages[0].Content, "[EMAIL_") {
		t.Error("should contain email token")
	}

	// Context should have mapping
	if len(ctx.AnonymizationMap) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(ctx.AnonymizationMap))
	}

	// Test de-anonymization
	resp := &core.ChatResponse{
		Choices: []core.Choice{
			{
				Message: core.Message{
					Role: "assistant",
					Content: func() string {
						for token := range ctx.AnonymizationMap {
							return "Got it, " + token
						}
						return ""
					}(),
				},
			},
		},
	}

	deResult := p.DeanonymizeChatResponse(resp, ctx)
	if !strings.Contains(deResult.Choices[0].Message.Content, "test@example.com") {
		t.Error("response should be de-anonymized")
	}
}

func TestProcessor_BothEnabled(t *testing.T) {
	cfg := Config{
		SystemPrompt: SystemPromptConfig{
			Enabled: true,
			Global: &SystemPromptRule{
				Prompt:   "Be helpful.",
				Position: PositionPrepend,
			},
		},
		Anonymization: AnonymizationConfig{
			Enabled:              true,
			Strategy:             StrategyToken,
			DeanonymizeResponses: true,
			Detectors: DetectorConfig{
				Email: true,
			},
		},
	}

	p := New(cfg)

	req := &core.ChatRequest{
		Model: "gpt-4",
		Messages: []core.Message{
			{Role: "user", Content: "Email: user@test.com"},
		},
	}

	result, ctx := p.ProcessChatRequest(req, "openai")

	// Should have 2 messages (system + user)
	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.Messages))
	}

	// System prompt should be first
	if result.Messages[0].Content != "Be helpful." {
		t.Error("system prompt not at start")
	}

	// User message should have anonymized email
	if strings.Contains(result.Messages[1].Content, "user@test.com") {
		t.Error("email should be anonymized")
	}

	// Context should have mapping
	if len(ctx.AnonymizationMap) != 1 {
		t.Error("should have anonymization mapping")
	}
}

func TestProcessor_ModelWhitelist(t *testing.T) {
	cfg := Config{
		Anonymization: AnonymizationConfig{
			Enabled:              true,
			Models:               []string{"gpt-3.5-turbo"}, // Only this model
			Strategy:             StrategyToken,
			DeanonymizeResponses: true,
			Detectors: DetectorConfig{
				Email: true,
			},
		},
	}

	p := New(cfg)

	// Request with whitelisted model
	req1 := &core.ChatRequest{
		Model: "gpt-3.5-turbo",
		Messages: []core.Message{
			{Role: "user", Content: "test@example.com"},
		},
	}

	result1, ctx1 := p.ProcessChatRequest(req1, "openai")
	if strings.Contains(result1.Messages[0].Content, "test@example.com") {
		t.Error("whitelisted model should have anonymization")
	}
	if len(ctx1.AnonymizationMap) == 0 {
		t.Error("should have anonymization mapping")
	}

	// Request with non-whitelisted model
	req2 := &core.ChatRequest{
		Model: "gpt-4",
		Messages: []core.Message{
			{Role: "user", Content: "test@example.com"},
		},
	}

	result2, ctx2 := p.ProcessChatRequest(req2, "openai")
	if !strings.Contains(result2.Messages[0].Content, "test@example.com") {
		t.Error("non-whitelisted model should NOT have anonymization")
	}
	if len(ctx2.AnonymizationMap) != 0 {
		t.Error("should NOT have anonymization mapping")
	}
}

func TestProcessor_WrapStreamForDeanonymization(t *testing.T) {
	cfg := Config{
		Anonymization: AnonymizationConfig{
			Enabled:              true,
			Strategy:             StrategyToken,
			DeanonymizeResponses: true,
			Detectors: DetectorConfig{
				Email: true,
			},
		},
	}

	p := New(cfg)

	// With tokens - should wrap
	ctx := &RequestContext{
		AnonymizationMap: map[string]string{
			"[EMAIL_1]": "test@example.com",
		},
	}

	input := `data: {"content":"[EMAIL_1]"}

`
	stream := io.NopCloser(strings.NewReader(input))
	wrapped := p.WrapStreamForDeanonymization(stream, ctx)

	output, _ := io.ReadAll(wrapped)
	if strings.Contains(string(output), "[EMAIL_1]") {
		t.Error("wrapped stream should de-anonymize tokens")
	}

	// Without tokens - should return original
	emptyCtx := &RequestContext{
		AnonymizationMap: map[string]string{},
	}

	stream2 := io.NopCloser(strings.NewReader(input))
	notWrapped := p.WrapStreamForDeanonymization(stream2, emptyCtx)

	// Should be the same reader (not wrapped)
	if notWrapped != stream2 {
		t.Error("empty context should return original stream")
	}
}

func TestProcessor_DeanonymizeDisabled(t *testing.T) {
	cfg := Config{
		Anonymization: AnonymizationConfig{
			Enabled:              true,
			Strategy:             StrategyToken,
			DeanonymizeResponses: false, // Disabled
			Detectors: DetectorConfig{
				Email: true,
			},
		},
	}

	p := New(cfg)

	req := &core.ChatRequest{
		Model: "gpt-4",
		Messages: []core.Message{
			{Role: "user", Content: "test@example.com"},
		},
	}

	_, ctx := p.ProcessChatRequest(req, "openai")

	resp := &core.ChatResponse{
		Choices: []core.Choice{
			{
				Message: core.Message{
					Role: "assistant",
					Content: func() string {
						for token := range ctx.AnonymizationMap {
							return token
						}
						return ""
					}(),
				},
			},
		},
	}

	// De-anonymization is disabled, so response should be unchanged
	deResult := p.DeanonymizeChatResponse(resp, ctx)
	if deResult != resp {
		t.Error("with de-anonymization disabled, should return original response")
	}
}
