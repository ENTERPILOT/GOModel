package core

import "testing"

func TestParseModelSelector(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		provider      string
		wantModel     string
		wantProvider  string
		wantQualified string
		wantErr       bool
	}{
		{
			name:          "plain model",
			model:         "gpt-4o",
			wantModel:     "gpt-4o",
			wantProvider:  "",
			wantQualified: "gpt-4o",
		},
		{
			name:          "prefixed model",
			model:         "openai/gpt-4o",
			wantModel:     "gpt-4o",
			wantProvider:  "openai",
			wantQualified: "openai/gpt-4o",
		},
		{
			name:          "provider field",
			model:         "gpt-4o",
			provider:      "openai",
			wantModel:     "gpt-4o",
			wantProvider:  "openai",
			wantQualified: "openai/gpt-4o",
		},
		{
			name:     "provider conflict",
			model:    "openai/gpt-4o",
			provider: "anthropic",
			wantErr:  true,
		},
		{
			name:    "missing model",
			model:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector, err := ParseModelSelector(tt.model, tt.provider)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if selector.Model != tt.wantModel {
				t.Fatalf("Model = %q, want %q", selector.Model, tt.wantModel)
			}
			if selector.Provider != tt.wantProvider {
				t.Fatalf("Provider = %q, want %q", selector.Provider, tt.wantProvider)
			}
			if selector.QualifiedModel() != tt.wantQualified {
				t.Fatalf("QualifiedModel = %q, want %q", selector.QualifiedModel(), tt.wantQualified)
			}
		})
	}
}
