package core

import "encoding/json"

// UnmarshalJSON validates chat request message content while preserving multimodal payloads.
func (m *Message) UnmarshalJSON(data []byte) error {
	var raw struct {
		Role      string          `json:"role"`
		Content   json.RawMessage `json:"content"`
		ToolCalls []ToolCall      `json:"tool_calls,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	content, err := UnmarshalMessageContent(raw.Content)
	if err != nil {
		return err
	}

	m.Role = raw.Role
	m.Content = content
	m.ToolCalls = raw.ToolCalls
	return nil
}

// MarshalJSON ensures only supported chat request message content shapes are emitted.
func (m Message) MarshalJSON() ([]byte, error) {
	content, err := NormalizeMessageContent(m.Content)
	if err != nil {
		return nil, err
	}

	return json.Marshal(struct {
		Role      string     `json:"role"`
		Content   any        `json:"content"`
		ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	}{
		Role:      m.Role,
		Content:   content,
		ToolCalls: m.ToolCalls,
	})
}
