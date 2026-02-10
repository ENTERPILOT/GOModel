package guardrails

import (
	"context"
	"testing"

	"gomodel/internal/core"
)

func TestNewSystemPromptGuardrail_InvalidMode(t *testing.T) {
	_, err := NewSystemPromptGuardrail("test", "bad", "content")
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestNewSystemPromptGuardrail_EmptyContent(t *testing.T) {
	_, err := NewSystemPromptGuardrail("test", SystemPromptInject, "")
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestNewSystemPromptGuardrail_ValidModes(t *testing.T) {
	for _, mode := range []SystemPromptMode{SystemPromptInject, SystemPromptOverride, SystemPromptDecorator} {
		g, err := NewSystemPromptGuardrail("my-guardrail", mode, "test")
		if err != nil {
			t.Fatalf("unexpected error for mode %q: %v", mode, err)
		}
		if g.Name() != "my-guardrail" {
			t.Errorf("expected name 'my-guardrail', got %q", g.Name())
		}
	}
}

func TestNewSystemPromptGuardrail_EmptyNameDefaults(t *testing.T) {
	g, err := NewSystemPromptGuardrail("", SystemPromptInject, "content")
	if err != nil {
		t.Fatal(err)
	}
	if g.Name() != "system_prompt" {
		t.Errorf("expected default name 'system_prompt', got %q", g.Name())
	}
}

func TestSystemPrompt_Inject_NoExistingSystem(t *testing.T) {
	g, _ := NewSystemPromptGuardrail("test", SystemPromptInject, "injected system prompt")
	req := &core.ChatRequest{
		Model: "gpt-4",
		Messages: []core.Message{
			{Role: "user", Content: "hello"},
		},
	}

	result, err := g.ProcessChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.Messages))
	}
	if result.Messages[0].Role != "system" || result.Messages[0].Content != "injected system prompt" {
		t.Errorf("expected system message first, got %+v", result.Messages[0])
	}
	if result.Messages[1].Role != "user" {
		t.Errorf("expected user message second, got %+v", result.Messages[1])
	}
}

func TestSystemPrompt_Inject_ExistingSystem(t *testing.T) {
	g, _ := NewSystemPromptGuardrail("test", SystemPromptInject, "injected system prompt")
	req := &core.ChatRequest{
		Model: "gpt-4",
		Messages: []core.Message{
			{Role: "system", Content: "original system"},
			{Role: "user", Content: "hello"},
		},
	}

	result, err := g.ProcessChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages (unchanged), got %d", len(result.Messages))
	}
	if result.Messages[0].Content != "original system" {
		t.Errorf("inject should not change existing system message, got %q", result.Messages[0].Content)
	}
}

func TestSystemPrompt_Override_NoExistingSystem(t *testing.T) {
	g, _ := NewSystemPromptGuardrail("test", SystemPromptOverride, "override prompt")
	req := &core.ChatRequest{
		Model: "gpt-4",
		Messages: []core.Message{
			{Role: "user", Content: "hello"},
		},
	}

	result, err := g.ProcessChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.Messages))
	}
	if result.Messages[0].Role != "system" || result.Messages[0].Content != "override prompt" {
		t.Errorf("expected override system message, got %+v", result.Messages[0])
	}
}

func TestSystemPrompt_Override_ExistingSystem(t *testing.T) {
	g, _ := NewSystemPromptGuardrail("test", SystemPromptOverride, "override prompt")
	req := &core.ChatRequest{
		Model: "gpt-4",
		Messages: []core.Message{
			{Role: "system", Content: "original system"},
			{Role: "user", Content: "hello"},
			{Role: "system", Content: "another system"},
		},
	}

	result, err := g.ProcessChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Should have: override system + user (both original system messages removed)
	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.Messages))
	}
	if result.Messages[0].Role != "system" || result.Messages[0].Content != "override prompt" {
		t.Errorf("expected override system message, got %+v", result.Messages[0])
	}
	if result.Messages[1].Role != "user" {
		t.Errorf("expected user message, got %+v", result.Messages[1])
	}
}

func TestSystemPrompt_Decorator_NoExistingSystem(t *testing.T) {
	g, _ := NewSystemPromptGuardrail("test", SystemPromptDecorator, "prefix")
	req := &core.ChatRequest{
		Model: "gpt-4",
		Messages: []core.Message{
			{Role: "user", Content: "hello"},
		},
	}

	result, err := g.ProcessChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.Messages))
	}
	if result.Messages[0].Role != "system" || result.Messages[0].Content != "prefix" {
		t.Errorf("decorator with no existing system should add one, got %+v", result.Messages[0])
	}
}

func TestSystemPrompt_Decorator_ExistingSystem(t *testing.T) {
	g, _ := NewSystemPromptGuardrail("test", SystemPromptDecorator, "prefix")
	req := &core.ChatRequest{
		Model: "gpt-4",
		Messages: []core.Message{
			{Role: "system", Content: "original"},
			{Role: "user", Content: "hello"},
		},
	}

	result, err := g.ProcessChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.Messages))
	}
	expected := "prefix\noriginal"
	if result.Messages[0].Content != expected {
		t.Errorf("expected decorated content %q, got %q", expected, result.Messages[0].Content)
	}
}

func TestSystemPrompt_Decorator_OnlyFirstSystemDecorated(t *testing.T) {
	g, _ := NewSystemPromptGuardrail("test", SystemPromptDecorator, "prefix")
	req := &core.ChatRequest{
		Model: "gpt-4",
		Messages: []core.Message{
			{Role: "system", Content: "first"},
			{Role: "user", Content: "hello"},
			{Role: "system", Content: "second"},
		},
	}

	result, err := g.ProcessChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if result.Messages[0].Content != "prefix\nfirst" {
		t.Errorf("first system should be decorated, got %q", result.Messages[0].Content)
	}
	if result.Messages[2].Content != "second" {
		t.Errorf("second system should be untouched, got %q", result.Messages[2].Content)
	}
}

func TestSystemPrompt_DoesNotMutateOriginal(t *testing.T) {
	g, _ := NewSystemPromptGuardrail("test", SystemPromptOverride, "new")
	original := &core.ChatRequest{
		Model: "gpt-4",
		Messages: []core.Message{
			{Role: "system", Content: "original"},
			{Role: "user", Content: "hello"},
		},
	}

	result, err := g.ProcessChat(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}

	// Original should be untouched
	if original.Messages[0].Content != "original" {
		t.Error("original request was mutated")
	}
	if result.Messages[0].Content != "new" {
		t.Error("result should have new system message")
	}
}

// --- Responses API tests ---

func TestSystemPrompt_Responses_Inject_NoInstructions(t *testing.T) {
	g, _ := NewSystemPromptGuardrail("test", SystemPromptInject, "injected")
	req := &core.ResponsesRequest{Model: "gpt-4", Input: "hello"}

	result, err := g.ProcessResponses(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Instructions != "injected" {
		t.Errorf("expected 'injected', got %q", result.Instructions)
	}
}

func TestSystemPrompt_Responses_Inject_ExistingInstructions(t *testing.T) {
	g, _ := NewSystemPromptGuardrail("test", SystemPromptInject, "injected")
	req := &core.ResponsesRequest{Model: "gpt-4", Input: "hello", Instructions: "existing"}

	result, err := g.ProcessResponses(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Instructions != "existing" {
		t.Errorf("inject should not change existing instructions, got %q", result.Instructions)
	}
}

func TestSystemPrompt_Responses_Override(t *testing.T) {
	g, _ := NewSystemPromptGuardrail("test", SystemPromptOverride, "override")
	req := &core.ResponsesRequest{Model: "gpt-4", Input: "hello", Instructions: "existing"}

	result, err := g.ProcessResponses(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Instructions != "override" {
		t.Errorf("expected 'override', got %q", result.Instructions)
	}
}

func TestSystemPrompt_Responses_Decorator_ExistingInstructions(t *testing.T) {
	g, _ := NewSystemPromptGuardrail("test", SystemPromptDecorator, "prefix")
	req := &core.ResponsesRequest{Model: "gpt-4", Input: "hello", Instructions: "existing"}

	result, err := g.ProcessResponses(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	expected := "prefix\nexisting"
	if result.Instructions != expected {
		t.Errorf("expected %q, got %q", expected, result.Instructions)
	}
}

func TestSystemPrompt_Responses_Decorator_NoInstructions(t *testing.T) {
	g, _ := NewSystemPromptGuardrail("test", SystemPromptDecorator, "prefix")
	req := &core.ResponsesRequest{Model: "gpt-4", Input: "hello"}

	result, err := g.ProcessResponses(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Instructions != "prefix" {
		t.Errorf("expected 'prefix', got %q", result.Instructions)
	}
}

func TestSystemPrompt_Responses_DoesNotMutateOriginal(t *testing.T) {
	g, _ := NewSystemPromptGuardrail("test", SystemPromptOverride, "new")
	original := &core.ResponsesRequest{Model: "gpt-4", Input: "hello", Instructions: "original"}

	result, err := g.ProcessResponses(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}
	if original.Instructions != "original" {
		t.Error("original request was mutated")
	}
	if result.Instructions != "new" {
		t.Error("result should have new instructions")
	}
}

func TestNewSystemPromptGuardrail_UnicodeNames(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"safety prompt"},           // spaces
		{"compliance check v2"},     // spaces and digits
		{"проверка безопасности"},   // Cyrillic with space
		{"安全検査"},                // CJK (Chinese + Japanese)
		{"sécurité-modèle"},        // accented Latin
		{"🛡️ guardrail"},           // emoji
	}
	for _, tc := range tests {
		g, err := NewSystemPromptGuardrail(tc.name, SystemPromptInject, "content")
		if err != nil {
			t.Errorf("unexpected error for name %q: %v", tc.name, err)
			continue
		}
		if g.Name() != tc.name {
			t.Errorf("expected name %q, got %q", tc.name, g.Name())
		}
	}
}

func TestSystemPrompt_PreservesOtherFields(t *testing.T) {
	g, _ := NewSystemPromptGuardrail("test", SystemPromptInject, "system")
	temp := 0.7
	maxTok := 100
	req := &core.ChatRequest{
		Model:       "gpt-4",
		Temperature: &temp,
		MaxTokens:   &maxTok,
		Messages:    []core.Message{{Role: "user", Content: "hello"}},
		Stream:      true,
		Reasoning:   &core.Reasoning{Effort: "high"},
	}

	result, err := g.ProcessChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if result.Model != "gpt-4" {
		t.Errorf("model not preserved")
	}
	if result.Temperature == nil || *result.Temperature != 0.7 {
		t.Errorf("temperature not preserved")
	}
	if result.MaxTokens == nil || *result.MaxTokens != 100 {
		t.Errorf("max_tokens not preserved")
	}
	if !result.Stream {
		t.Errorf("stream not preserved")
	}
	if result.Reasoning == nil || result.Reasoning.Effort != "high" {
		t.Errorf("reasoning not preserved")
	}
}
