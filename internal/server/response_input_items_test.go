package server

import (
	"encoding/json"
	"testing"

	"gomodel/internal/core"
)

func TestNormalizedResponseInputItemsSkipsNilDefaultInput(t *testing.T) {
	var input *core.ResponsesInputElement
	req := &core.ResponsesRequest{Input: input}

	items := normalizedResponseInputItems("resp_1", req)
	if len(items) != 0 {
		t.Fatalf("len(items) = %d, want 0", len(items))
	}
}

func TestNormalizedResponseInputRawSkipsNullObject(t *testing.T) {
	item := normalizedResponseInputRaw("resp_1", 0, json.RawMessage("null"))
	if len(item) != 0 {
		t.Fatalf("len(item) = %d, want 0", len(item))
	}
}
