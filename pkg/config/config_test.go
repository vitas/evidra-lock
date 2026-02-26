package config

import "testing"

func TestNormalizeEnvironment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"prod", "production"},
		{"prd", "production"},
		{"PRD", "production"},
		{"Prod", "production"},
		{"production", "production"},
		{"stg", "staging"},
		{"stage", "staging"},
		{"staging", "staging"},
		{"STG", "staging"},
		{"dev", "dev"},
		{"custom", "custom"},
		{"", ""},
		{"  prod  ", "production"},
		{"  ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := NormalizeEnvironment(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeEnvironment(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
