package core

import (
	"encoding/json"
	"testing"
)

func TestResponsesRequestUnmarshalJSON_StringInput(t *testing.T) {
	var req ResponsesRequest
	if err := json.Unmarshal([]byte(`{"model":"gpt-4o-mini","input":"hello"}`), &req); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if req.Model != "gpt-4o-mini" {
		t.Fatalf("Model = %q, want gpt-4o-mini", req.Model)
	}
	input, ok := req.Input.(string)
	if !ok || input != "hello" {
		t.Fatalf("Input = %#v, want string hello", req.Input)
	}
}

func TestResponsesRequestUnmarshalJSON_ArrayInput(t *testing.T) {
	var req ResponsesRequest
	if err := json.Unmarshal([]byte(`{"model":"gpt-4o-mini","input":[{"role":"user","content":[{"type":"input_text","text":"hello"}]}]}`), &req); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	input, ok := req.Input.([]interface{})
	if !ok || len(input) != 1 {
		t.Fatalf("Input = %#v, want []interface{} len=1", req.Input)
	}
}

func TestResponsesRequestUnmarshalJSON_PreservesToolCallingControls(t *testing.T) {
	var req ResponsesRequest
	if err := json.Unmarshal([]byte(`{
		"model":"gpt-4o-mini",
		"input":"hello",
		"tool_choice":{"type":"function","function":{"name":"lookup_weather"}},
		"parallel_tool_calls":false
	}`), &req); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	toolChoice, ok := req.ToolChoice.(map[string]interface{})
	if !ok {
		t.Fatalf("ToolChoice = %#v, want object", req.ToolChoice)
	}
	if typ, _ := toolChoice["type"].(string); typ != "function" {
		t.Fatalf("ToolChoice.type = %#v, want function", toolChoice["type"])
	}
	if req.ParallelToolCalls == nil || *req.ParallelToolCalls {
		t.Fatalf("ParallelToolCalls = %#v, want false", req.ParallelToolCalls)
	}
}

func TestResponsesRequestMarshalJSON_PreservesInput(t *testing.T) {
	body, err := json.Marshal(ResponsesRequest{
		Model: "gpt-4o-mini",
		Input: []interface{}{
			map[string]interface{}{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{
						"type": "input_text",
						"text": "hello",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	inputRaw, ok := decoded["input"]
	if !ok {
		t.Fatalf("marshal output missing input: %s", string(body))
	}

	input, ok := inputRaw.([]interface{})
	if !ok || len(input) != 1 {
		t.Fatalf("decoded input = %#v, want []interface{} len=1", inputRaw)
	}

	firstMsg, ok := input[0].(map[string]interface{})
	if !ok {
		t.Fatalf("first input item = %#v, want object", input[0])
	}
	if role, _ := firstMsg["role"].(string); role != "user" {
		t.Fatalf("first input role = %#v, want user", firstMsg["role"])
	}

	contentRaw, ok := firstMsg["content"]
	if !ok {
		t.Fatalf("first input missing content: %#v", firstMsg)
	}
	content, ok := contentRaw.([]interface{})
	if !ok || len(content) != 1 {
		t.Fatalf("first input content = %#v, want []interface{} len=1", contentRaw)
	}

	firstPart, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("first content part = %#v, want object", content[0])
	}
	if typ, _ := firstPart["type"].(string); typ != "input_text" {
		t.Fatalf("first content type = %#v, want input_text", firstPart["type"])
	}
	if text, _ := firstPart["text"].(string); text != "hello" {
		t.Fatalf("first content text = %#v, want hello", firstPart["text"])
	}
}

func TestResponsesRequestMarshalJSON_PreservesToolCallingControls(t *testing.T) {
	parallelToolCalls := false
	body, err := json.Marshal(ResponsesRequest{
		Model: "gpt-4o-mini",
		Input: "hello",
		ToolChoice: map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": "lookup_weather",
			},
		},
		ParallelToolCalls: &parallelToolCalls,
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	toolChoice, ok := decoded["tool_choice"].(map[string]interface{})
	if !ok {
		t.Fatalf("decoded tool_choice = %#v, want object", decoded["tool_choice"])
	}
	if typ, _ := toolChoice["type"].(string); typ != "function" {
		t.Fatalf("decoded tool_choice.type = %#v, want function", toolChoice["type"])
	}
	parallel, ok := decoded["parallel_tool_calls"].(bool)
	if !ok || parallel {
		t.Fatalf("decoded parallel_tool_calls = %#v, want false", decoded["parallel_tool_calls"])
	}
}

func TestResponsesRequestMarshalJSON_PreservesTypedInputItemContent(t *testing.T) {
	body, err := json.Marshal(ResponsesRequest{
		Model: "gpt-4o-mini",
		Input: []ResponsesInputItem{
			{
				Role:    "user",
				Content: "hello",
			},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	input, ok := decoded["input"].([]interface{})
	if !ok || len(input) != 1 {
		t.Fatalf("decoded input = %#v, want []interface{} len=1", decoded["input"])
	}

	first, ok := input[0].(map[string]interface{})
	if !ok {
		t.Fatalf("decoded first input item = %#v, want object", input[0])
	}
	if role, _ := first["role"].(string); role != "user" {
		t.Fatalf("decoded role = %#v, want user", first["role"])
	}
	if content, _ := first["content"].(string); content != "hello" {
		t.Fatalf("decoded content = %#v, want hello", first["content"])
	}
}
