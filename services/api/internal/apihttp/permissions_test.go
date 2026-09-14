package apihttp

import "testing"

func TestRolePermissions(t *testing.T) {
	tests := []struct {
		role       string
		admin      bool
		canAnalyze bool
	}{
		{role: "owner", admin: true, canAnalyze: true},
		{role: "admin", admin: true, canAnalyze: true},
		{role: "analyst", admin: false, canAnalyze: true},
		{role: "viewer", admin: false, canAnalyze: false},
		{role: "", admin: false, canAnalyze: false},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			if got := isAdmin(tt.role); got != tt.admin {
				t.Fatalf("isAdmin(%q) = %v, want %v", tt.role, got, tt.admin)
			}
			if got := canAnalyze(tt.role); got != tt.canAnalyze {
				t.Fatalf("canAnalyze(%q) = %v, want %v", tt.role, got, tt.canAnalyze)
			}
		})
	}
}
