package ops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPolicyPathsExist(t *testing.T) {
	if DefaultPolicyPath != "./policy/profiles/ops-v0.1/policy.rego" {
		t.Fatalf("expected DefaultPolicyPath to point to structured profile, got %q", DefaultPolicyPath)
	}
	if DefaultDataPath != "./policy/profiles/ops-v0.1/data.json" {
		t.Fatalf("expected DefaultDataPath to point to structured data, got %q", DefaultDataPath)
	}

	root := filepath.Join("..", "..")
	policyPath := filepath.Join(root, "policy", "profiles", "ops-v0.1", "policy.rego")
	dataPath := filepath.Join(root, "policy", "profiles", "ops-v0.1", "data.json")
	if _, err := os.Stat(policyPath); err != nil {
		t.Fatalf("default policy file missing: %v", err)
	}
	if _, err := os.Stat(dataPath); err != nil {
		t.Fatalf("default policy data missing: %v", err)
	}
}

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
