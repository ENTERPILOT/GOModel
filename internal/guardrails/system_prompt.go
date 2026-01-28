package guardrails

import (
	"gomodel/internal/core"
)

// SystemPromptInjector handles system prompt injection into requests.
type SystemPromptInjector struct {
	config SystemPromptConfig
}

// NewSystemPromptInjector creates a new SystemPromptInjector.
func NewSystemPromptInjector(cfg SystemPromptConfig) *SystemPromptInjector {
	return &SystemPromptInjector{config: cfg}
}

// resolveRule finds the applicable system prompt rule for a model and provider.
// Precedence: model-specific > provider-specific > global
func (s *SystemPromptInjector) resolveRule(model, providerType string) *SystemPromptRule {
	// Model-specific rule has highest precedence
	if rule, ok := s.config.Models[model]; ok {
		return &rule
	}

	// Provider-specific rule
	if rule, ok := s.config.Providers[providerType]; ok {
		return &rule
	}

	// Global rule
	return s.config.Global
}

// ProcessChatRequest applies system prompt injection to a ChatRequest.
// Returns a new request with the modified messages (does not mutate input).
func (s *SystemPromptInjector) ProcessChatRequest(req *core.ChatRequest, providerType string) *core.ChatRequest {
	rule := s.resolveRule(req.Model, providerType)
	if rule == nil || rule.Prompt == "" {
		return req
	}

	// Create a copy of the request with cloned messages
	newReq := *req
	newReq.Messages = s.injectSystemPrompt(req.Messages, rule)

	return &newReq
}

// ProcessResponsesRequest applies system prompt injection to a ResponsesRequest.
// For ResponsesRequest, the system prompt goes into the Instructions field.
func (s *SystemPromptInjector) ProcessResponsesRequest(req *core.ResponsesRequest, providerType string) *core.ResponsesRequest {
	rule := s.resolveRule(req.Model, providerType)
	if rule == nil || rule.Prompt == "" {
		return req
	}

	// Create a copy of the request
	newReq := *req

	position := rule.Position
	if position == "" {
		position = PositionPrepend
	}

	switch position {
	case PositionPrepend:
		if req.Instructions != "" && rule.PreserveUserSystem {
			newReq.Instructions = rule.Prompt + "\n\n" + req.Instructions
		} else if req.Instructions != "" && !rule.PreserveUserSystem {
			newReq.Instructions = rule.Prompt
		} else {
			newReq.Instructions = rule.Prompt
		}
	case PositionAppend:
		if req.Instructions != "" && rule.PreserveUserSystem {
			newReq.Instructions = req.Instructions + "\n\n" + rule.Prompt
		} else if req.Instructions != "" && !rule.PreserveUserSystem {
			newReq.Instructions = rule.Prompt
		} else {
			newReq.Instructions = rule.Prompt
		}
	case PositionReplace:
		newReq.Instructions = rule.Prompt
	}

	return &newReq
}

// injectSystemPrompt injects the system prompt into the messages array.
func (s *SystemPromptInjector) injectSystemPrompt(messages []core.Message, rule *SystemPromptRule) []core.Message {
	position := rule.Position
	if position == "" {
		position = PositionPrepend
	}

	// Find existing system message
	systemIdx := -1
	for i, msg := range messages {
		if msg.Role == "system" {
			systemIdx = i
			break
		}
	}

	// Create the new messages slice
	newMessages := make([]core.Message, 0, len(messages)+1)

	switch position {
	case PositionPrepend:
		if systemIdx >= 0 {
			// There's an existing system message
			if rule.PreserveUserSystem {
				// Prepend to existing: combined system message first
				for i, msg := range messages {
					if i == systemIdx {
						newMessages = append(newMessages, core.Message{
							Role:    "system",
							Content: rule.Prompt + "\n\n" + msg.Content,
						})
					} else {
						newMessages = append(newMessages, msg)
					}
				}
			} else {
				// Replace existing with our prompt
				for i, msg := range messages {
					if i == systemIdx {
						newMessages = append(newMessages, core.Message{
							Role:    "system",
							Content: rule.Prompt,
						})
					} else {
						newMessages = append(newMessages, msg)
					}
				}
			}
		} else {
			// No existing system message - add one at the beginning
			newMessages = append(newMessages, core.Message{
				Role:    "system",
				Content: rule.Prompt,
			})
			newMessages = append(newMessages, messages...)
		}

	case PositionAppend:
		if systemIdx >= 0 {
			// There's an existing system message
			if rule.PreserveUserSystem {
				// Append to existing: existing content + our prompt
				for i, msg := range messages {
					if i == systemIdx {
						newMessages = append(newMessages, core.Message{
							Role:    "system",
							Content: msg.Content + "\n\n" + rule.Prompt,
						})
					} else {
						newMessages = append(newMessages, msg)
					}
				}
			} else {
				// Replace existing with our prompt
				for i, msg := range messages {
					if i == systemIdx {
						newMessages = append(newMessages, core.Message{
							Role:    "system",
							Content: rule.Prompt,
						})
					} else {
						newMessages = append(newMessages, msg)
					}
				}
			}
		} else {
			// No existing system message - add one at the beginning (even for append mode)
			newMessages = append(newMessages, core.Message{
				Role:    "system",
				Content: rule.Prompt,
			})
			newMessages = append(newMessages, messages...)
		}

	case PositionReplace:
		// Replace any existing system message or add new one
		if systemIdx >= 0 {
			for i, msg := range messages {
				if i == systemIdx {
					newMessages = append(newMessages, core.Message{
						Role:    "system",
						Content: rule.Prompt,
					})
				} else {
					newMessages = append(newMessages, msg)
				}
			}
		} else {
			newMessages = append(newMessages, core.Message{
				Role:    "system",
				Content: rule.Prompt,
			})
			newMessages = append(newMessages, messages...)
		}
	}

	return newMessages
}
