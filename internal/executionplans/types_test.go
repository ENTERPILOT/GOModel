package executionplans

import "testing"

func TestNormalizeScope_RejectsColonDelimitedFields(t *testing.T) {
	t.Parallel()

	tests := []Scope{
		{Provider: "openai:beta"},
		{Provider: "openai", Model: "gpt:5"},
	}

	for _, scope := range tests {
		scope := scope
		t.Run(scope.Provider+"|"+scope.Model, func(t *testing.T) {
			t.Parallel()

			_, _, err := normalizeScope(scope)
			if err == nil {
				t.Fatal("normalizeScope() error = nil, want validation error")
			}
			if !IsValidationError(err) {
				t.Fatalf("normalizeScope() error = %T, want validation error", err)
			}
		})
	}
}
