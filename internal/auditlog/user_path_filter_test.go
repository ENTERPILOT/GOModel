package auditlog

import "testing"

func TestAuditUserPathSubtreePattern(t *testing.T) {
	tests := []struct {
		name     string
		userPath string
		want     string
	}{
		{
			name:     "root matches full subtree",
			userPath: "/",
			want:     "/%",
		},
		{
			name:     "nested path appends descendant wildcard",
			userPath: "/team/a",
			want:     "/team/a/%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := auditUserPathSubtreePattern(tt.userPath); got != tt.want {
				t.Fatalf("auditUserPathSubtreePattern(%q) = %q, want %q", tt.userPath, got, tt.want)
			}
		})
	}
}
