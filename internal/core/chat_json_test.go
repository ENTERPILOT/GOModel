package core

import (
	"encoding/json"
	"testing"
)

func TestChatRequestJSON_RoundTripPreservesUnknownFields(t *testing.T) {
	body := []byte(`{
		"model":"gpt-4o-mini",
		"messages":[
			{
				"role":"user",
				"content":"hello",
				"x_message_meta":{"id":"msg-1"},
				"tool_calls":[
					{
						"id":"call_1",
						"type":"function",
						"x_tool_call":true,
						"function":{
							"name":"lookup_weather",
							"arguments":"{}",
							"x_function_meta":{"strict":true}
						}
					}
				]
			}
		],
		"tools":[
			{
				"type":"function",
				"function":{"name":"lookup_weather","parameters":{"type":"object"}},
				"x_tool_meta":"keep-me"
			}
		],
		"stream":true,
		"x_trace":{"id":"trace-1"}
	}`)

	wantExtra, err := extractUnknownJSONFields(body,
		"temperature",
		"max_tokens",
		"model",
		"provider",
		"messages",
		"tools",
		"tool_choice",
		"parallel_tool_calls",
		"stream",
		"stream_options",
		"reasoning",
	)
	if err != nil {
		t.Fatalf("extractUnknownJSONFields() error = %v", err)
	}

	var req ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if req.Model != "gpt-4o-mini" {
		t.Fatalf("Model = %q, want gpt-4o-mini", req.Model)
	}
	if req.ExtraFields["x_trace"] == nil || string(req.ExtraFields["x_trace"]) != string(wantExtra["x_trace"]) {
		t.Fatalf("ExtraFields[x_trace] = %s, want %s", req.ExtraFields["x_trace"], wantExtra["x_trace"])
	}
	if len(req.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(req.Messages))
	}
	if req.Messages[0].ExtraFields["x_message_meta"] == nil {
		t.Fatalf("message extras missing: %+v", req.Messages[0].ExtraFields)
	}
	if len(req.Messages[0].ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(req.Messages[0].ToolCalls))
	}
	if req.Messages[0].ToolCalls[0].ExtraFields["x_tool_call"] == nil {
		t.Fatalf("tool call extras missing: %+v", req.Messages[0].ToolCalls[0].ExtraFields)
	}
	if req.Messages[0].ToolCalls[0].Function.ExtraFields["x_function_meta"] == nil {
		t.Fatalf("function extras missing: %+v", req.Messages[0].ToolCalls[0].Function.ExtraFields)
	}
	if got := req.Tools[0]["x_tool_meta"]; got != "keep-me" {
		t.Fatalf("tools[0][x_tool_meta] = %#v, want keep-me", got)
	}

	roundTrip, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(roundTrip, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(roundTrip) error = %v", err)
	}
	if _, ok := decoded["x_trace"].(map[string]any); !ok {
		t.Fatalf("x_trace = %#v, want object", decoded["x_trace"])
	}

	messages, ok := decoded["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("messages = %#v, want len=1", decoded["messages"])
	}
	message := messages[0].(map[string]any)
	if _, ok := message["x_message_meta"].(map[string]any); !ok {
		t.Fatalf("x_message_meta = %#v, want object", message["x_message_meta"])
	}
	toolCalls := message["tool_calls"].([]any)
	toolCall := toolCalls[0].(map[string]any)
	if toolCall["x_tool_call"] != true {
		t.Fatalf("x_tool_call = %#v, want true", toolCall["x_tool_call"])
	}
	function := toolCall["function"].(map[string]any)
	if _, ok := function["x_function_meta"].(map[string]any); !ok {
		t.Fatalf("x_function_meta = %#v, want object", function["x_function_meta"])
	}

	tools := decoded["tools"].([]any)
	tool := tools[0].(map[string]any)
	if tool["x_tool_meta"] != "keep-me" {
		t.Fatalf("x_tool_meta = %#v, want keep-me", tool["x_tool_meta"])
	}
}
