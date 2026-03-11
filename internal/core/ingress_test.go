package core

import "testing"

func TestNewIngressFrame_DefensivelyCopiesMutableFields(t *testing.T) {
	routeParams := map[string]string{"provider": "openai"}
	queryParams := map[string][]string{"limit": {"5"}}
	headers := map[string][]string{"X-Test": {"a", "b"}}
	rawBody := []byte(`{"model":"gpt-5-mini"}`)
	traceMetadata := map[string]string{"Traceparent": "trace-1"}

	frame := NewIngressFrame(
		"POST",
		"/v1/chat/completions",
		routeParams,
		queryParams,
		headers,
		"application/json",
		rawBody,
		false,
		"req-123",
		traceMetadata,
	)

	routeParams["provider"] = "anthropic"
	queryParams["limit"][0] = "99"
	headers["X-Test"][0] = "mutated"
	rawBody[0] = '['
	traceMetadata["Traceparent"] = "trace-2"

	if got := frame.GetRouteParams()["provider"]; got != "openai" {
		t.Fatalf("GetRouteParams provider = %q, want openai", got)
	}
	if got := frame.GetQueryParams()["limit"][0]; got != "5" {
		t.Fatalf("GetQueryParams limit = %q, want 5", got)
	}
	if got := frame.GetHeaders()["X-Test"][0]; got != "a" {
		t.Fatalf("GetHeaders X-Test = %q, want a", got)
	}
	if got := string(frame.GetRawBody()); got != `{"model":"gpt-5-mini"}` {
		t.Fatalf("GetRawBody = %q, want original body", got)
	}
	if got := frame.GetTraceMetadata()["Traceparent"]; got != "trace-1" {
		t.Fatalf("GetTraceMetadata Traceparent = %q, want trace-1", got)
	}

	clonedHeaders := frame.GetHeaders()
	clonedHeaders["X-Test"][0] = "changed-again"
	if got := frame.GetHeaders()["X-Test"][0]; got != "a" {
		t.Fatalf("GetHeaders returned mutable state, got %q", got)
	}
}
