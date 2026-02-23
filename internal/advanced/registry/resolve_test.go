package registry

import (
	"errors"
	"testing"
)

func TestResolveOperationSuccess(t *testing.T) {
	def := newDeclarativeTestTool(t, "dummy", map[string]CLIOperationSpec{
		"one": {Args: []string{"one"}, Params: map[string]ParamRule{}},
	})
	reg := NewInMemoryRegistry([]ToolDefinition{def})

	got, err := ResolveOperation(reg, "dummy", "one")
	if err != nil {
		t.Fatalf("ResolveOperation() error = %v", err)
	}
	if got.Name != "dummy" {
		t.Fatalf("unexpected tool: %s", got.Name)
	}
}

func TestResolveOperationToolNotFound(t *testing.T) {
	reg := NewInMemoryRegistry(nil)

	_, err := ResolveOperation(reg, "missing", "any")
	if err == nil {
		t.Fatalf("expected error for missing tool")
	}
	var notFound ErrToolNotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("expected ErrToolNotFound, got %T", err)
	}
	if notFound.Tool != "missing" {
		t.Fatalf("expected tool name in error, got %v", notFound.Tool)
	}
}

func TestResolveOperationOperationNotFound(t *testing.T) {
	def := newDeclarativeTestTool(t, "dummy", map[string]CLIOperationSpec{
		"exists": {Args: []string{"exists"}, Params: map[string]ParamRule{}},
	})
	reg := NewInMemoryRegistry([]ToolDefinition{def})

	_, err := ResolveOperation(reg, "dummy", "missing")
	if err == nil {
		t.Fatalf("expected error for missing operation")
	}
	var opErr ErrOperationNotFound
	if !errors.As(err, &opErr) {
		t.Fatalf("expected ErrOperationNotFound, got %T", err)
	}
	if opErr.Tool != "dummy" || opErr.Operation != "missing" {
		t.Fatalf("unexpected error context: %+v", opErr)
	}
}

func newDeclarativeTestTool(t *testing.T, name string, operations map[string]CLIOperationSpec) ToolDefinition {
	t.Helper()
	spec := CLIToolSpec{
		Binary:     name,
		Operations: operations,
	}
	def, err := NewDeclarativeCLIToolDefinition(name, "test tool", spec)
	if err != nil {
		t.Fatalf("NewDeclarativeCLIToolDefinition() error = %v", err)
	}
	return def
}
