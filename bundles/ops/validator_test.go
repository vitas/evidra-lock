package ops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateFileScenarioPass(t *testing.T) {
	withRepoRoot(t, func() {
		path := filepath.Join("bundles", "ops", "examples", "scenario_pass.json")
		result, err := ValidateFile(path)
		if err != nil {
			t.Fatalf("ValidateFile returned error: %v", err)
		}
		if !result.Pass {
			t.Fatalf("expected scenario to PASS, got FAIL")
		}
		if result.RiskLevel == "" {
			t.Fatalf("expected risk level to be set")
		}
		if result.EvidenceID == "" {
			t.Fatalf("expected evidence ID")
		}
	})
}

func withRepoRoot(t *testing.T, fn func()) {
	t.Helper()
	root := filepath.Join("..", "..")
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir repo root: %v", err)
	}
	defer func() {
		_ = os.Chdir(orig)
	}()
	fn()
}
