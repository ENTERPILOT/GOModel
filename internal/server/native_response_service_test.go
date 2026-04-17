package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gomodel/internal/core"
)

func TestResponsesUtilityRoutesRejectNullBody(t *testing.T) {
	provider := &mockProvider{
		supportedModels: []string{"gpt-5-mini"},
		providerTypes: map[string]string{
			"gpt-5-mini": "mock",
		},
	}
	srv := New(provider, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses/input_tokens", strings.NewReader("null"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
	}
	if len(provider.capturedResponseUtilityReqs) != 0 {
		t.Fatalf("utility calls = %d, want 0", len(provider.capturedResponseUtilityReqs))
	}
}

func TestNativeResponseByProviderWrapsContextCancellation(t *testing.T) {
	provider := &mockProvider{
		providerTypes: map[string]string{
			"gpt-5-mini": "mock",
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := nativeResponseByProvider[*core.ResponsesResponse](ctx, provider, "", func(core.NativeResponseLifecycleRoutableProvider, string) (*core.ResponsesResponse, error) {
		t.Fatal("provider call should not run after context cancellation")
		return nil, nil
	})

	var gatewayErr *core.GatewayError
	if !errors.As(err, &gatewayErr) {
		t.Fatalf("error = %T %[1]v, want *core.GatewayError", err)
	}
	if gatewayErr.HTTPStatusCode() != http.StatusRequestTimeout {
		t.Fatalf("status = %d, want 408", gatewayErr.HTTPStatusCode())
	}
}
