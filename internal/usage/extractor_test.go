package usage

import (
	"testing"

	"gomodel/internal/core"
)

func TestExtractFromChatResponse(t *testing.T) {
	tests := []struct {
		name         string
		resp         *core.ChatResponse
		requestID    string
		provider     string
		endpoint     string
		wantNil      bool
		wantInput    int
		wantOutput   int
		wantTotal    int
		wantRawData  bool
		wantProvider string
		wantModel    string
	}{
		{
			name:     "nil response",
			resp:     nil,
			provider: "openai",
			wantNil:  true,
		},
		{
			name: "basic response",
			resp: &core.ChatResponse{
				ID:    "chatcmpl-123",
				Model: "gpt-4",
				Usage: core.Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				},
			},
			requestID:    "req-123",
			provider:     "openai",
			endpoint:     "/v1/chat/completions",
			wantInput:    100,
			wantOutput:   50,
			wantTotal:    150,
			wantProvider: "openai",
			wantModel:    "gpt-4",
		},
		{
			name: "response with raw usage",
			resp: &core.ChatResponse{
				ID:    "chatcmpl-456",
				Model: "gpt-4o",
				Usage: core.Usage{
					PromptTokens:     200,
					CompletionTokens: 100,
					TotalTokens:      300,
					RawUsage: map[string]any{
						"cached_tokens":    50,
						"reasoning_tokens": 25,
					},
				},
			},
			requestID:    "req-456",
			provider:     "openai",
			endpoint:     "/v1/chat/completions",
			wantInput:    200,
			wantOutput:   100,
			wantTotal:    300,
			wantRawData:  true,
			wantProvider: "openai",
			wantModel:    "gpt-4o",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := ExtractFromChatResponse(tt.resp, tt.requestID, tt.provider, tt.endpoint)

			if tt.wantNil {
				if entry != nil {
					t.Error("expected nil entry")
				}
				return
			}

			if entry == nil {
				t.Fatal("expected non-nil entry")
			}

			if entry.InputTokens != tt.wantInput {
				t.Errorf("InputTokens = %d, want %d", entry.InputTokens, tt.wantInput)
			}
			if entry.OutputTokens != tt.wantOutput {
				t.Errorf("OutputTokens = %d, want %d", entry.OutputTokens, tt.wantOutput)
			}
			if entry.TotalTokens != tt.wantTotal {
				t.Errorf("TotalTokens = %d, want %d", entry.TotalTokens, tt.wantTotal)
			}
			if entry.Provider != tt.wantProvider {
				t.Errorf("Provider = %s, want %s", entry.Provider, tt.wantProvider)
			}
			if entry.Model != tt.wantModel {
				t.Errorf("Model = %s, want %s", entry.Model, tt.wantModel)
			}
			if entry.RequestID != tt.requestID {
				t.Errorf("RequestID = %s, want %s", entry.RequestID, tt.requestID)
			}
			if entry.Endpoint != tt.endpoint {
				t.Errorf("Endpoint = %s, want %s", entry.Endpoint, tt.endpoint)
			}
			if tt.wantRawData && entry.RawData == nil {
				t.Error("expected RawData to be set")
			}
			if !tt.wantRawData && entry.RawData != nil {
				t.Error("expected RawData to be nil")
			}
		})
	}
}

func TestExtractFromResponsesResponse(t *testing.T) {
	tests := []struct {
		name       string
		resp       *core.ResponsesResponse
		requestID  string
		provider   string
		endpoint   string
		wantNil    bool
		wantInput  int
		wantOutput int
		wantTotal  int
	}{
		{
			name:     "nil response",
			resp:     nil,
			provider: "openai",
			wantNil:  true,
		},
		{
			name: "response with nil usage",
			resp: &core.ResponsesResponse{
				ID:    "resp-123",
				Model: "gpt-4",
				Usage: nil,
			},
			requestID:  "req-123",
			provider:   "openai",
			endpoint:   "/v1/responses",
			wantInput:  0,
			wantOutput: 0,
			wantTotal:  0,
		},
		{
			name: "response with usage",
			resp: &core.ResponsesResponse{
				ID:    "resp-456",
				Model: "gpt-4",
				Usage: &core.ResponsesUsage{
					InputTokens:  100,
					OutputTokens: 50,
					TotalTokens:  150,
				},
			},
			requestID:  "req-456",
			provider:   "openai",
			endpoint:   "/v1/responses",
			wantInput:  100,
			wantOutput: 50,
			wantTotal:  150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := ExtractFromResponsesResponse(tt.resp, tt.requestID, tt.provider, tt.endpoint)

			if tt.wantNil {
				if entry != nil {
					t.Error("expected nil entry")
				}
				return
			}

			if entry == nil {
				t.Fatal("expected non-nil entry")
			}

			if entry.InputTokens != tt.wantInput {
				t.Errorf("InputTokens = %d, want %d", entry.InputTokens, tt.wantInput)
			}
			if entry.OutputTokens != tt.wantOutput {
				t.Errorf("OutputTokens = %d, want %d", entry.OutputTokens, tt.wantOutput)
			}
			if entry.TotalTokens != tt.wantTotal {
				t.Errorf("TotalTokens = %d, want %d", entry.TotalTokens, tt.wantTotal)
			}
		})
	}
}

func TestExtractFromSSEUsage(t *testing.T) {
	entry := ExtractFromSSEUsage(
		"chatcmpl-789",
		100, 50, 150,
		map[string]any{"cached_tokens": 25},
		"req-789", "gpt-4", "openai", "/v1/chat/completions",
	)

	if entry == nil {
		t.Fatal("expected non-nil entry")
	}

	if entry.ProviderID != "chatcmpl-789" {
		t.Errorf("ProviderID = %s, want chatcmpl-789", entry.ProviderID)
	}
	if entry.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", entry.InputTokens)
	}
	if entry.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", entry.OutputTokens)
	}
	if entry.TotalTokens != 150 {
		t.Errorf("TotalTokens = %d, want 150", entry.TotalTokens)
	}
	if entry.RawData == nil {
		t.Error("expected RawData to be set")
	}
	if entry.RawData["cached_tokens"] != 25 {
		t.Errorf("RawData[cached_tokens] = %v, want 25", entry.RawData["cached_tokens"])
	}
}

func TestExtractFromSSEUsageEmptyRawData(t *testing.T) {
	entry := ExtractFromSSEUsage(
		"chatcmpl-789",
		100, 50, 150,
		nil, // empty raw data
		"req-789", "gpt-4", "openai", "/v1/chat/completions",
	)

	if entry == nil {
		t.Fatal("expected non-nil entry")
	}

	if entry.RawData != nil {
		t.Error("expected RawData to be nil")
	}
}
