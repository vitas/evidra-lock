package validate_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/scenario"
	"samebits.com/evidra-mcp/pkg/validate"
)

var profileDir = filepath.Join("..", "..", "policy", "profiles", "ops-v0.1")

// safeOpts returns Options backed by the real ops-v0.1 profile and a
// temporary evidence dir. Use as the baseline for happy-path tests.
func safeOpts(t *testing.T) validate.Options {
	t.Helper()
	return validate.Options{
		PolicyPath:  filepath.Join(profileDir, "policy.rego"),
		DataPath:    filepath.Join(profileDir, "data.json"),
		EvidenceDir: t.TempDir(),
	}
}

// safeScenario returns a minimal scenario that the ops-v0.1 policy allows.
func safeScenario() scenario.Scenario {
	return scenario.Scenario{
		ScenarioID: "test-scenario",
		Actor:      scenario.Actor{Type: "human", ID: "u1", Origin: "test"},
		Source:     "test",
		Timestamp:  time.Now().UTC(),
		Actions: []scenario.Action{
			{
				Kind:   "kubectl.apply",
				Target: map[string]interface{}{"namespace": "default"},
			},
		},
	}
}

func TestEvaluateInvocationErrInvalidInput(t *testing.T) {
	// An empty ToolInvocation fails ValidateStructure.
	_, err := validate.EvaluateInvocation(context.Background(), invocation.ToolInvocation{}, validate.Options{})
	if err == nil {
		t.Fatal("expected error for empty invocation, got nil")
	}
	if !errors.Is(err, validate.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput; got %v", err)
	}
}

func TestEvaluateScenarioErrPolicyFailure(t *testing.T) {
	opts := validate.Options{
		PolicyPath:  "nonexistent.rego",
		DataPath:    "nonexistent.json",
		EvidenceDir: t.TempDir(),
	}
	_, err := validate.EvaluateScenario(context.Background(), safeScenario(), opts)
	if err == nil {
		t.Fatal("expected error for bad policy path, got nil")
	}
	if !errors.Is(err, validate.ErrPolicyFailure) {
		t.Errorf("expected ErrPolicyFailure; got %v", err)
	}
}

func TestEvaluateScenarioErrEvidenceWrite(t *testing.T) {
	// Use a path whose parent component is a regular file.
	// os.Stat returns ENOTDIR (not ErrNotExist), causing detectStoreMode to
	// propagate the error through the store init path.
	parentAsFile := filepath.Join(t.TempDir(), "notadir")
	f, err := os.Create(parentAsFile)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	f.Close()

	opts := safeOpts(t)
	opts.EvidenceDir = filepath.Join(parentAsFile, "evidence") // ENOTDIR

	_, err = validate.EvaluateScenario(context.Background(), safeScenario(), opts)
	if err == nil {
		t.Fatal("expected error when evidence dir is a file, got nil")
	}
	if !errors.Is(err, validate.ErrEvidenceWrite) {
		t.Errorf("expected ErrEvidenceWrite; got %v", err)
	}
}
