package guardrails

import (
	"context"
	"fmt"
)

// SystemPromptMode defines how the system prompt guardrail modifies messages.
type SystemPromptMode string

const (
	// SystemPromptInject adds a system message only if none exists.
	SystemPromptInject SystemPromptMode = "inject"

	// SystemPromptOverride replaces all existing system messages with the configured one.
	SystemPromptOverride SystemPromptMode = "override"

	// SystemPromptDecorator prepends the configured content to the first existing
	// system message (separated by a newline), or adds a new system message if none exists.
	SystemPromptDecorator SystemPromptMode = "decorator"
)

// SystemPromptGuardrail injects, overrides, or decorates system messages.
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

// Process applies the system prompt guardrail to a normalized message list.
func (g *SystemPromptGuardrail) Process(_ context.Context, msgs []Message) ([]Message, error) {
	switch g.mode {
	case SystemPromptInject:
		return g.inject(msgs), nil
	case SystemPromptOverride:
		return g.override(msgs), nil
	case SystemPromptDecorator:
		return g.decorate(msgs), nil
	default:
		return msgs, nil
	}
}

// inject adds a system message at the beginning only if no system message exists.
func (g *SystemPromptGuardrail) inject(msgs []Message) []Message {
	for _, m := range msgs {
		if m.Role == "system" {
			return msgs // already has a system message, leave untouched
		}
	}
	result := make([]Message, 0, len(msgs)+1)
	result = append(result, Message{Role: "system", Content: g.content})
	result = append(result, msgs...)
	return result
}

// override replaces all system messages with a single one at the beginning.
func (g *SystemPromptGuardrail) override(msgs []Message) []Message {
	result := make([]Message, 0, len(msgs)+1)
	result = append(result, Message{Role: "system", Content: g.content})
	for _, m := range msgs {
		if m.Role != "system" {
			result = append(result, m)
		}
	}
	return result
}

// decorate prepends the configured content to the first system message,
// or adds a new system message if none exists.
func (g *SystemPromptGuardrail) decorate(msgs []Message) []Message {
	found := false
	result := make([]Message, len(msgs))
	copy(result, msgs)

	for i, m := range result {
		if m.Role == "system" && !found {
			result[i].Content = g.content + "\n" + m.Content
			found = true
		}
	}

	if !found {
		prepended := make([]Message, 0, len(result)+1)
		prepended = append(prepended, Message{Role: "system", Content: g.content})
		prepended = append(prepended, result...)
		return prepended
	}
	return result
}
