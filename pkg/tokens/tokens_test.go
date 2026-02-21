package tokens

import "testing"

func TestValidateTemplate(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		allowed   map[string]bool
		shouldErr bool
	}{
		{
			name:      "allows known required token",
			template:  "{{x}}",
			allowed:   map[string]bool{"x": true},
			shouldErr: false,
		},
		{
			name:      "allows known optional token",
			template:  "{{x?}}",
			allowed:   map[string]bool{"x": true},
			shouldErr: false,
		},
		{
			name:      "rejects unknown required token",
			template:  "{{x}}",
			allowed:   map[string]bool{"y": true},
			shouldErr: true,
		},
		{
			name:      "rejects unknown optional token",
			template:  "{{x?}}",
			allowed:   map[string]bool{"y": true},
			shouldErr: true,
		},
		{
			name:      "rejects malformed empty token",
			template:  "{{}}",
			allowed:   map[string]bool{"x": true},
			shouldErr: true,
		},
		{
			name:      "rejects malformed question token",
			template:  "{{?}}",
			allowed:   map[string]bool{"x": true},
			shouldErr: true,
		},
		{
			name:      "accepts whitespace around token name",
			template:  "{{ x }}",
			allowed:   map[string]bool{"x": true},
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTemplate(tt.template, tt.allowed)
			if tt.shouldErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.shouldErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestExpandTemplate(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		values    map[string]string
		want      string
		shouldErr bool
	}{
		{
			name:      "replaces required token",
			template:  "{{x}}",
			values:    map[string]string{"x": "value"},
			want:      "value",
			shouldErr: false,
		},
		{
			name:      "removes optional token when missing",
			template:  "pre{{x?}}post",
			values:    map[string]string{},
			want:      "prepost",
			shouldErr: false,
		},
		{
			name:      "errors when required token missing",
			template:  "{{x}}",
			values:    map[string]string{},
			shouldErr: true,
		},
		{
			name:      "supports inline required and optional tokens",
			template:  "cmd --arg={{x}} --opt={{y?}}",
			values:    map[string]string{"x": "one"},
			want:      "cmd --arg=one --opt=",
			shouldErr: false,
		},
		{
			name:      "multiple tokens in one template",
			template:  "{{a}}-{{b}}-{{c?}}",
			values:    map[string]string{"a": "1", "b": "2"},
			want:      "1-2-",
			shouldErr: false,
		},
		{
			name:      "whole-string optional token",
			template:  "{{x?}}",
			values:    map[string]string{},
			want:      "",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandTemplate(tt.template, tt.values)
			if tt.shouldErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected expanded template: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestExtractTokens(t *testing.T) {
	tests := []struct {
		name         string
		template     string
		wantRequired []string
		wantOptional []string
		shouldErr    bool
	}{
		{
			name:         "extracts required and optional tokens",
			template:     "cmd {{a}} {{b?}}",
			wantRequired: []string{"a"},
			wantOptional: []string{"b"},
			shouldErr:    false,
		},
		{
			name:         "dedupes duplicate tokens",
			template:     "{{a}} {{a}} {{b?}} {{b?}}",
			wantRequired: []string{"a"},
			wantOptional: []string{"b"},
			shouldErr:    false,
		},
		{
			name:      "handles inline and whole-string tokens",
			template:  "-chdir={{dir}} {{mode?}}",
			shouldErr: false,
			wantRequired: []string{
				"dir",
			},
			wantOptional: []string{
				"mode",
			},
		},
		{
			name:      "rejects malformed token",
			template:  "{{}}",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			required, optional, err := ExtractTokens(tt.template)
			if tt.shouldErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			assertStringSliceEqual(t, required, tt.wantRequired)
			assertStringSliceEqual(t, optional, tt.wantOptional)
		})
	}
}

func TestPlaceholders(t *testing.T) {
	got, err := Placeholders("{{a}}-{{b?}}-{{a}}")
	if err != nil {
		t.Fatalf("Placeholders() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 placeholders, got %d", len(got))
	}
	if got[0].Name != "a" || got[0].Optional {
		t.Fatalf("unexpected first placeholder: %+v", got[0])
	}
	if got[1].Name != "b" || !got[1].Optional {
		t.Fatalf("unexpected second placeholder: %+v", got[1])
	}
}

func assertStringSliceEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got=%v want=%v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("mismatch at %d: got=%v want=%v", i, got, want)
		}
	}
}
