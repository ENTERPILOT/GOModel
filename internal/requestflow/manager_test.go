package requestflow

import (
	"testing"
	"time"

	"gomodel/config"
)

func TestResolve_MergesBaseAppAndModelPlans(t *testing.T) {
	appRetries := 5
	modelRetries := 1
	initialBackoff := Duration(2 * time.Second)
	failoverEnabled := true

	manager := NewManager(Options{
		BaseRetry: config.DefaultRetryConfig(),
		BaseRules: []GuardrailRule{{
			Name:  "base",
			Type:  "system_prompt",
			Order: 0,
			SystemPrompt: SystemPromptSettings{
				Mode:    "inject",
				Content: "base",
			},
		}},
		YAMLDefs: []*Definition{
			{
				ID:       "app",
				Name:     "app defaults",
				Enabled:  true,
				Priority: 10,
				Spec: PlanSpec{
					Guardrails: GuardrailSpec{Mode: "append", Rules: []GuardrailRule{{
						Name:         "app",
						Type:         "system_prompt",
						Order:        1,
						SystemPrompt: SystemPromptSettings{Mode: "inject", Content: "app"},
					}}},
					Retry: RetryPolicy{MaxRetries: &appRetries},
				},
			},
			{
				ID:       "model",
				Name:     "model override",
				Enabled:  true,
				Priority: 100,
				Match:    MatchCriteria{Model: "gpt-4o"},
				Spec: PlanSpec{
					Guardrails: GuardrailSpec{Mode: "replace", Rules: []GuardrailRule{{
						Name:         "model",
						Type:         "system_prompt",
						Order:        2,
						SystemPrompt: SystemPromptSettings{Mode: "override", Content: "model"},
					}}},
					Retry:    RetryPolicy{MaxRetries: &modelRetries, InitialBackoff: &initialBackoff},
					Failover: FailoverPolicy{Enabled: &failoverEnabled, Strategy: "same_model"},
				},
			},
		},
	})

	plan, err := manager.Resolve(ResolveContext{Model: "gpt-4o", Endpoint: "/v1/chat/completions"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got := len(plan.Matches); got != 2 {
		t.Fatalf("expected 2 matches, got %d", got)
	}
	if got := len(plan.Guardrails); got != 1 {
		t.Fatalf("expected model replace to leave 1 guardrail, got %d", got)
	}
	if got := plan.Guardrails[0].Name; got != "model" {
		t.Fatalf("expected model guardrail, got %q", got)
	}
	if plan.Retry.MaxRetries != modelRetries {
		t.Fatalf("expected model retry override %d, got %d", modelRetries, plan.Retry.MaxRetries)
	}
	if plan.Retry.InitialBackoff != 2*time.Second {
		t.Fatalf("expected initial backoff 2s, got %v", plan.Retry.InitialBackoff)
	}
	if !plan.FailoverEnabled {
		t.Fatal("expected failover to be enabled")
	}
	if plan.FailoverStrategy != "same_model" {
		t.Fatalf("expected failover strategy same_model, got %q", plan.FailoverStrategy)
	}
}

func TestHashAPIKey_Stable(t *testing.T) {
	first := HashAPIKey("Bearer secret-token")
	second := HashAPIKey("Bearer secret-token")
	third := HashAPIKey("Bearer another-token")
	if first == "" {
		t.Fatal("expected non-empty hash")
	}
	if first != second {
		t.Fatalf("expected stable hash, got %q and %q", first, second)
	}
	if first == third {
		t.Fatalf("expected different hashes for different tokens, got %q", first)
	}
}
