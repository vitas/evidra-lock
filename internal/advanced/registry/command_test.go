package registry

import (
	"errors"
	"testing"
)

func TestBuildDeclarativeCLIArgsGolden(t *testing.T) {
	spec := CLIToolSpec{
		Binary: "kubectl",
		Operations: map[string]CLIOperationSpec{
			"apply": {
				Args: []string{"apply", "-f", "{{file}}", "-n", "{{namespace}}"},
				Params: map[string]ParamRule{
					"file":      {Type: "string", Required: true},
					"namespace": {Type: "string", Required: true},
				},
			},
		},
	}

	argv, err := BuildDeclarativeCLIArgs("kubectl", spec, "apply", map[string]interface{}{
		"file":      "infra.yaml",
		"namespace": "prod",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"kubectl", "apply", "-f", "infra.yaml", "-n", "prod"}
	assertSlicesEqual(t, expected, argv)
}

func TestBuildDeclarativeCLIArgsConvertsInts(t *testing.T) {
	spec := CLIToolSpec{
		Binary: "cli",
		Operations: map[string]CLIOperationSpec{
			"count": {
				Args: []string{"count", "--limit", "{{limit}}"},
				Params: map[string]ParamRule{
					"limit": {Type: "int", Required: true},
				},
			},
		},
	}

	argv, err := BuildDeclarativeCLIArgs("cli", spec, "count", map[string]interface{}{
		"limit": 12,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"cli", "count", "--limit", "12"}
	assertSlicesEqual(t, expected, argv)
}

func assertSlicesEqual(t *testing.T, expected, got []string) {
	t.Helper()
	if len(expected) != len(got) {
		t.Fatalf("argv length mismatch; want %d got %d (%v)", len(expected), len(got), got)
	}
	for i := range expected {
		if expected[i] != got[i] {
			t.Fatalf("argv mismatch at %d: want %q got %q", i, expected[i], got[i])
		}
	}
}

func TestBuildDeclarativeCLIArgsBoolParam(t *testing.T) {
	spec := CLIToolSpec{
		Binary: "cli",
		Operations: map[string]CLIOperationSpec{
			"set": {
				Args: []string{"set", "--enabled={{enabled}}"},
				Params: map[string]ParamRule{
					"enabled": {Type: "bool", Required: true},
				},
			},
		},
	}

	argv, err := BuildDeclarativeCLIArgs("cli", spec, "set", map[string]interface{}{"enabled": true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"cli", "set", "--enabled=true"}
	assertSlicesEqual(t, want, argv)
}

func TestBuildDeclarativeCLIArgsRejectsDisallowedString(t *testing.T) {
	spec := CLIToolSpec{
		Binary: "cli",
		Operations: map[string]CLIOperationSpec{
			"write": {
				Args: []string{"write", "--path={{path}}"},
				Params: map[string]ParamRule{
					"path": {Type: "string", Required: true},
				},
			},
		},
	}

	_, err := BuildDeclarativeCLIArgs("cli", spec, "write", map[string]interface{}{"path": "line\nbreak"})
	if err == nil {
		t.Fatalf("expected error for newline in string")
	}
	var invalid ErrInvalidParams
	if !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidParams, got %T", err)
	}
	if invalid.Reason != "contains disallowed characters" {
		t.Fatalf("unexpected reason: %q", invalid.Reason)
	}
}

func TestNewDeclarativeCLIToolDefinitionValidation(t *testing.T) {
	cases := []struct {
		name string
		spec CLIToolSpec
	}{
		{"empty-name", CLIToolSpec{Binary: "bin", Operations: map[string]CLIOperationSpec{"run": {Args: []string{"run"}, Params: map[string]ParamRule{}}}}},
		{"empty-binary", CLIToolSpec{}},
		{"missing-operations", CLIToolSpec{Binary: "cli"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewDeclarativeCLIToolDefinition("", "schema", tc.spec)
			if err == nil {
				t.Fatalf("expected validation error for %s", tc.name)
			}
		})
	}
}
