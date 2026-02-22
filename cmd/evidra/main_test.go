package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateUsage(t *testing.T) {
	code, _, stderr := runFromRepoRoot(t, []string{"validate"})
	if code != 2 {
		t.Fatalf("expected exit code 2 for missing file, got %d", code)
	}
	if !strings.Contains(stderr, "usage: evidra validate <file>") {
		t.Fatalf("expected usage message, got: %s", stderr)
	}
}

func TestValidateScenarioPass(t *testing.T) {
	scenario := filepath.Join("bundles", "ops", "examples", "scenario_pass.json")
	code, out, errOut := runFromRepoRoot(t, []string{"validate", scenario})
	if code != 0 {
		t.Fatalf("expected exit code 0 for passing scenario, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "Decision: PASS") {
		t.Fatalf("expected PASS decision, got: %s", out)
	}
}

func TestVersionCommand(t *testing.T) {
	code, out, errOut := runFromRepoRoot(t, []string{"version"})
	if code != 0 {
		t.Fatalf("expected code 0, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "Version:") {
		t.Fatalf("expected version output, got: %s", out)
	}
}

func runFromRepoRoot(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	root := filepath.Join("..", "..")
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(orig)
	}()

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}
