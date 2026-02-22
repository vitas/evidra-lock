package registry

import (
	"errors"
	"testing"
)

func TestBuildDeclarativeCLIArgsMissingRequiredParam(t *testing.T) {
	spec := CLIToolSpec{
		Binary: "cli",
		Operations: map[string]CLIOperationSpec{
			"deploy": {
				Args: []string{"deploy", "{{resource}}"},
				Params: map[string]ParamRule{
					"resource": {Type: "string", Required: true},
				},
			},
		},
	}

	_, err := BuildDeclarativeCLIArgs("cli", spec, "deploy", nil)
	if err == nil {
		t.Fatalf("expected error for missing required param")
	}
	var invalid ErrInvalidParams
	if !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidParams, got %T", err)
	}
	if invalid.Param != "resource" {
		t.Fatalf("expected missing param context, got %q", invalid.Param)
	}
	if invalid.Reason != "missing required param" {
		t.Fatalf("unexpected reason: %q", invalid.Reason)
	}
}

func TestBuildDeclarativeCLIArgsIntTypeMismatch(t *testing.T) {
	spec := CLIToolSpec{
		Binary: "cli",
		Operations: map[string]CLIOperationSpec{
			"count": {
				Args: []string{"count", "{{limit}}"},
				Params: map[string]ParamRule{
					"limit": {Type: "int", Required: true},
				},
			},
		},
	}

	_, err := BuildDeclarativeCLIArgs("cli", spec, "count", map[string]interface{}{
		"limit": 1.5,
	})
	if err == nil {
		t.Fatalf("expected type mismatch error for int param")
	}
	var invalid ErrInvalidParams
	if !errors.As(err, &invalid) {
		t.Fatalf("expected ErrInvalidParams, got %T", err)
	}
	if invalid.Param != "limit" {
		t.Fatalf("expected limit context, got %q", invalid.Param)
	}
	if invalid.Reason != "must be int" {
		t.Fatalf("unexpected reason: %q", invalid.Reason)
	}
}

func TestBuildDeclarativeCLIArgsOptionalTokenSkipped(t *testing.T) {
	spec := CLIToolSpec{
		Binary: "cli",
		Operations: map[string]CLIOperationSpec{
			"run": {
				Args: []string{"run", "--message", "{{msg}}", "--flag={{flag?}}"},
				Params: map[string]ParamRule{
					"msg":  {Type: "string", Required: true},
					"flag": {Type: "string", Required: false},
				},
			},
		},
	}

	argv, err := BuildDeclarativeCLIArgs("cli", spec, "run", map[string]interface{}{"msg": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"cli", "run", "--message", "hello"}
	if len(argv) != len(expected) {
		t.Fatalf("unexpected argv length: %v", argv)
	}
	for i := range expected {
		if argv[i] != expected[i] {
			t.Fatalf("argv mismatch at %d: got %q want %q", i, argv[i], expected[i])
		}
	}
}
