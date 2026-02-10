package guardrails

import (
	"context"
	"fmt"

	"gomodel/internal/core"
)

// SystemPromptMode defines how the system prompt guardrail modifies requests.
type SystemPromptMode string

const (
	// SystemPromptInject adds a system message only if none exists in the request.
	SystemPromptInject SystemPromptMode = "inject"

	// SystemPromptOverride replaces all existing system messages with the configured one.
	SystemPromptOverride SystemPromptMode = "override"

	// SystemPromptDecorator prepends the configured content to the first existing
	// system message (separated by a newline), or adds a new system message if none exists.
	SystemPromptDecorator SystemPromptMode = "decorator"
)

// SystemPromptGuardrail injects, overrides, or decorates system messages in requests.
type SystemPromptGuardrail struct {
	name    string
	mode    SystemPromptMode
	content string
}

// NewSystemPromptGuardrail creates a new system prompt guardrail instance.
// name identifies this instance (e.g. "safety-prompt", "compliance-check").
// mode must be "inject", "override", or "decorator".
// content is the system prompt text to apply.
func NewSystemPromptGuardrail(name string, mode SystemPromptMode, content string) (*SystemPromptGuardrail, error) {
	switch mode {
	case SystemPromptInject, SystemPromptOverride, SystemPromptDecorator:
	default:
		return nil, fmt.Errorf("invalid system prompt mode: %q (must be inject, override, or decorator)", mode)
	}
	if content == "" {
		return nil, fmt.Errorf("system prompt content cannot be empty")
	}
	if name == "" {
		name = "system_prompt"
	}
	return &SystemPromptGuardrail{
		name:    name,
		mode:    mode,
		content: content,
	}, nil
}

// Name returns this instance's name.
func (g *SystemPromptGuardrail) Name() string {
	return g.name
}

// ProcessChat applies the system prompt guardrail to a chat completion request.
func (g *SystemPromptGuardrail) ProcessChat(ctx context.Context, req *core.ChatRequest) (*core.ChatRequest, error) {
	messages := g.applyToMessages(req.Messages)
	// Return a shallow copy with the new messages slice
	return &core.ChatRequest{
		Temperature:   req.Temperature,
		MaxTokens:     req.MaxTokens,
		Model:         req.Model,
		Messages:      messages,
		Stream:        req.Stream,
		StreamOptions: req.StreamOptions,
		Reasoning:     req.Reasoning,
	}, nil
}

// ProcessResponses applies the system prompt guardrail to a Responses API request.
// For the Responses API, the "instructions" field serves as the system prompt.
func (g *SystemPromptGuardrail) ProcessResponses(ctx context.Context, req *core.ResponsesRequest) (*core.ResponsesRequest, error) {
	instructions := g.applyToInstructions(req.Instructions)
	return &core.ResponsesRequest{
		Model:           req.Model,
		Input:           req.Input,
		Instructions:    instructions,
		Tools:           req.Tools,
		Temperature:     req.Temperature,
		MaxOutputTokens: req.MaxOutputTokens,
		Stream:          req.Stream,
		StreamOptions:   req.StreamOptions,
		Metadata:        req.Metadata,
		Reasoning:       req.Reasoning,
	}, nil
}

// applyToMessages applies the mode logic to a slice of chat messages.
func (g *SystemPromptGuardrail) applyToMessages(messages []core.Message) []core.Message {
	switch g.mode {
	case SystemPromptInject:
		return g.injectMessages(messages)
	case SystemPromptOverride:
		return g.overrideMessages(messages)
	case SystemPromptDecorator:
		return g.decorateMessages(messages)
	default:
		return messages
	}
}

// injectMessages adds a system message at the beginning only if no system message exists.
func (g *SystemPromptGuardrail) injectMessages(messages []core.Message) []core.Message {
	for _, m := range messages {
		if m.Role == "system" {
			return messages // already has a system message, leave untouched
		}
	}
	// Prepend system message
	result := make([]core.Message, 0, len(messages)+1)
	result = append(result, core.Message{Role: "system", Content: g.content})
	result = append(result, messages...)
	return result
}

// overrideMessages replaces all system messages with a single one at the beginning.
func (g *SystemPromptGuardrail) overrideMessages(messages []core.Message) []core.Message {
	result := make([]core.Message, 0, len(messages)+1)
	result = append(result, core.Message{Role: "system", Content: g.content})
	for _, m := range messages {
		if m.Role != "system" {
			result = append(result, m)
		}
	}
	return result
}

// decorateMessages prepends the configured content to the first system message,
// or adds a new system message if none exists.
func (g *SystemPromptGuardrail) decorateMessages(messages []core.Message) []core.Message {
	found := false
	result := make([]core.Message, len(messages))
	copy(result, messages)

	for i, m := range result {
		if m.Role == "system" && !found {
			result[i].Content = g.content + "\n" + m.Content
			found = true
		}
	}

	if !found {
		// No system message found; prepend one
		prepended := make([]core.Message, 0, len(result)+1)
		prepended = append(prepended, core.Message{Role: "system", Content: g.content})
		prepended = append(prepended, result...)
		return prepended
	}
	return result
}

// applyToInstructions applies the mode logic to the Responses API instructions field.
func (g *SystemPromptGuardrail) applyToInstructions(instructions string) string {
	switch g.mode {
	case SystemPromptInject:
		if instructions != "" {
			return instructions // already has instructions, leave untouched
		}
		return g.content
	case SystemPromptOverride:
		return g.content
	case SystemPromptDecorator:
		if instructions != "" {
			return g.content + "\n" + instructions
		}
		return g.content
	default:
		return instructions
	}
}
