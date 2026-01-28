package guardrails

import (
	"strings"
	"testing"

	"gomodel/internal/core"
)

func TestAnonymizer_ShouldAnonymize(t *testing.T) {
	tests := []struct {
		name   string
		models []string
		model  string
		want   bool
	}{
		{
			name:   "empty models list - anonymize all",
			models: nil,
			model:  "gpt-4",
			want:   true,
		},
		{
			name:   "model in whitelist",
			models: []string{"gpt-4", "gpt-3.5-turbo"},
			model:  "gpt-4",
			want:   true,
		},
		{
			name:   "model not in whitelist",
			models: []string{"gpt-4", "gpt-3.5-turbo"},
			model:  "claude-3-opus",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AnonymizationConfig{
				Enabled: true,
				Models:  tt.models,
			}
			a := NewAnonymizer(cfg)
			if got := a.ShouldAnonymize(tt.model); got != tt.want {
				t.Errorf("ShouldAnonymize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnonymizer_AnonymizeText(t *testing.T) {
	cfg := AnonymizationConfig{
		Enabled:  true,
		Strategy: StrategyToken,
		Detectors: DetectorConfig{
			Email:      true,
			Phone:      true,
			SSN:        true,
			CreditCard: true,
			IPAddress:  true,
		},
	}
	a := NewAnonymizer(cfg)

	tests := []struct {
		name        string
		input       string
		wantTokens  []string // Token prefixes we expect to find
		wantRemoved []string // Original values that should be removed
	}{
		{
			name:        "email address",
			input:       "Contact me at test@example.com",
			wantTokens:  []string{"[EMAIL_"},
			wantRemoved: []string{"test@example.com"},
		},
		{
			name:        "phone number with dashes",
			input:       "Call me at 123-456-7890",
			wantTokens:  []string{"[PHONE_"},
			wantRemoved: []string{"123-456-7890"},
		},
		{
			name:        "phone number with parentheses",
			input:       "Call me at (123) 456-7890",
			wantTokens:  []string{"[PHONE_"},
			wantRemoved: []string{"(123) 456-7890"},
		},
		{
			name:        "SSN with dashes",
			input:       "My SSN is 123-45-6789",
			wantTokens:  []string{"[SSN_"},
			wantRemoved: []string{"123-45-6789"},
		},
		{
			name:        "credit card with dashes",
			input:       "Card: 1234-5678-9012-3456",
			wantTokens:  []string{"[CC_"},
			wantRemoved: []string{"1234-5678-9012-3456"},
		},
		{
			name:        "IP address",
			input:       "Server IP: 192.168.1.100",
			wantTokens:  []string{"[IP_"},
			wantRemoved: []string{"192.168.1.100"},
		},
		{
			name:        "multiple PII types",
			input:       "Email: user@test.com, Phone: 555-123-4567",
			wantTokens:  []string{"[EMAIL_", "[PHONE_"},
			wantRemoved: []string{"user@test.com", "555-123-4567"},
		},
		{
			name:        "no PII",
			input:       "Hello, how are you?",
			wantTokens:  nil,
			wantRemoved: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenMap := make(map[string]string)
			result := a.anonymizeText(tt.input, tokenMap)

			// Check that expected tokens are present
			for _, tokenPrefix := range tt.wantTokens {
				if !strings.Contains(result, tokenPrefix) {
					t.Errorf("result should contain token prefix %q, got: %s", tokenPrefix, result)
				}
			}

			// Check that original values are removed
			for _, orig := range tt.wantRemoved {
				if strings.Contains(result, orig) {
					t.Errorf("result should not contain original value %q, got: %s", orig, result)
				}
			}

			// Check tokenMap has correct mappings
			for token, original := range tokenMap {
				found := false
				for _, removed := range tt.wantRemoved {
					if original == removed {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("tokenMap contains unexpected mapping: %s -> %s", token, original)
				}
			}
		})
	}
}

func TestAnonymizer_AnonymizeChatRequest(t *testing.T) {
	cfg := AnonymizationConfig{
		Enabled:  true,
		Strategy: StrategyToken,
		Detectors: DetectorConfig{
			Email: true,
		},
	}
	a := NewAnonymizer(cfg)

	req := &core.ChatRequest{
		Model: "gpt-4",
		Messages: []core.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "My email is test@example.com"},
		},
	}

	result, tokenMap := a.AnonymizeChatRequest(req)

	// Original should be unchanged
	if req.Messages[1].Content != "My email is test@example.com" {
		t.Error("original request should not be modified")
	}

	// Result should have tokenized email
	if strings.Contains(result.Messages[1].Content, "test@example.com") {
		t.Error("result should not contain original email")
	}
	if !strings.Contains(result.Messages[1].Content, "[EMAIL_") {
		t.Error("result should contain email token")
	}

	// Token map should have the mapping
	if len(tokenMap) != 1 {
		t.Errorf("expected 1 token mapping, got %d", len(tokenMap))
	}

	for token, original := range tokenMap {
		if original != "test@example.com" {
			t.Errorf("expected original to be email, got %s", original)
		}
		if !strings.HasPrefix(token, "[EMAIL_") {
			t.Errorf("expected token to start with [EMAIL_, got %s", token)
		}
	}
}

func TestAnonymizer_Deanonymize(t *testing.T) {
	cfg := AnonymizationConfig{
		Enabled:              true,
		Strategy:             StrategyToken,
		DeanonymizeResponses: true,
		Detectors: DetectorConfig{
			Email: true,
		},
	}
	a := NewAnonymizer(cfg)

	tokenMap := map[string]string{
		"[EMAIL_1]": "test@example.com",
		"[EMAIL_2]": "other@test.org",
	}

	resp := &core.ChatResponse{
		Choices: []core.Choice{
			{
				Message: core.Message{
					Role:    "assistant",
					Content: "I see your emails are [EMAIL_1] and [EMAIL_2]",
				},
			},
		},
	}

	result := a.DeanonymizeChatResponse(resp, tokenMap)

	expected := "I see your emails are test@example.com and other@test.org"
	if result.Choices[0].Message.Content != expected {
		t.Errorf("deanonymized content = %q, want %q", result.Choices[0].Message.Content, expected)
	}
}

func TestAnonymizer_Strategies(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		check    func(token string) bool
	}{
		{
			name:     "token strategy",
			strategy: StrategyToken,
			check: func(token string) bool {
				// Token format: [TYPE_hexid]
				return strings.HasPrefix(token, "[EMAIL_") && strings.HasSuffix(token, "]")
			},
		},
		{
			name:     "hash strategy",
			strategy: StrategyHash,
			check: func(token string) bool {
				// Hash format: [TYPE_8charhash]
				return strings.HasPrefix(token, "[EMAIL_") && len(token) == len("[EMAIL_12345678]")
			},
		},
		{
			name:     "mask strategy",
			strategy: StrategyMask,
			check: func(token string) bool {
				// Mask format: [TYPE_x***y]
				return strings.HasPrefix(token, "[EMAIL_t***m]")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AnonymizationConfig{
				Enabled:  true,
				Strategy: tt.strategy,
				Detectors: DetectorConfig{
					Email: true,
				},
			}
			a := NewAnonymizer(cfg)

			tokenMap := make(map[string]string)
			result := a.anonymizeText("test@example.com", tokenMap)

			if len(tokenMap) != 1 {
				t.Fatalf("expected 1 token, got %d", len(tokenMap))
			}

			for token := range tokenMap {
				if !tt.check(token) {
					t.Errorf("token format check failed for %q", token)
				}
				if !strings.Contains(result, token) {
					t.Errorf("result should contain token %q", token)
				}
			}
		})
	}
}

func TestAnonymizer_DetectorConfig(t *testing.T) {
	tests := []struct {
		name      string
		detectors DetectorConfig
		input     string
		wantMatch bool
	}{
		{
			name:      "email enabled - matches",
			detectors: DetectorConfig{Email: true},
			input:     "test@example.com",
			wantMatch: true,
		},
		{
			name:      "email disabled - no match",
			detectors: DetectorConfig{Email: false},
			input:     "test@example.com",
			wantMatch: false,
		},
		{
			name:      "phone enabled - matches",
			detectors: DetectorConfig{Phone: true},
			input:     "123-456-7890",
			wantMatch: true,
		},
		{
			name:      "only email enabled - phone not matched",
			detectors: DetectorConfig{Email: true, Phone: false},
			input:     "123-456-7890",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AnonymizationConfig{
				Enabled:   true,
				Strategy:  StrategyToken,
				Detectors: tt.detectors,
			}
			a := NewAnonymizer(cfg)

			tokenMap := make(map[string]string)
			result := a.anonymizeText(tt.input, tokenMap)

			hasMatch := len(tokenMap) > 0
			if hasMatch != tt.wantMatch {
				t.Errorf("hasMatch = %v, want %v. Result: %s", hasMatch, tt.wantMatch, result)
			}
		})
	}
}

func TestAnonymizer_SameValueSameToken(t *testing.T) {
	cfg := AnonymizationConfig{
		Enabled:  true,
		Strategy: StrategyToken,
		Detectors: DetectorConfig{
			Email: true,
		},
	}
	a := NewAnonymizer(cfg)

	tokenMap := make(map[string]string)
	input := "Email 1: test@example.com, Email 2: test@example.com"
	result := a.anonymizeText(input, tokenMap)

	// Should only have one token for the same email
	if len(tokenMap) != 1 {
		t.Errorf("expected 1 token for duplicate value, got %d", len(tokenMap))
	}

	// Count occurrences of the token
	var token string
	for t := range tokenMap {
		token = t
	}

	count := strings.Count(result, token)
	if count != 2 {
		t.Errorf("expected token to appear 2 times, got %d", count)
	}
}
