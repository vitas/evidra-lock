package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"samebits.com/evidra/pkg/config"
)

func TestValidateUsage(t *testing.T) {
	code, _, stderr := runFromRepoRoot(t, []string{"validate"})
	if code != 2 {
		t.Fatalf("expected exit code 2 for missing file, got %d", code)
	}
	if !strings.Contains(stderr, "usage: evidra validate") {
		t.Fatalf("expected usage message, got: %s", stderr)
	}
}

func TestValidateScenarioPass(t *testing.T) {
	scenario := filepath.Join("examples", "terraform_plan_pass.json")
	code, out, errOut := runFromRepoRoot(t, []string{"validate", scenario})
	if code != 0 {
		t.Fatalf("expected exit code 0 for passing scenario, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "Decision: PASS") {
		t.Fatalf("expected PASS decision, got: %s", out)
	}
}

func TestValidateExplainOutput(t *testing.T) {
	scenario := filepath.Join("examples", "kubernetes_kube_system_block.json")
	code, out, errOut := runFromRepoRoot(t, []string{"validate", "--explain", scenario})
	if code != 2 {
		t.Fatalf("expected exit code 2 for deny, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "Explanation:") {
		t.Fatalf("expected explanation section, got: %s", out)
	}
	if !strings.Contains(out, "Rule IDs:") {
		t.Fatalf("expected rule IDs, got: %s", out)
	}
	if !strings.Contains(out, "Hints:") {
		t.Fatalf("expected hints section, got: %s", out)
	}

}

func TestValidateFailStructuredOutput(t *testing.T) {
	scenario := filepath.Join("examples", "terraform_public_exposure_fail.json")
	code, out, errOut := runFromRepoRoot(t, []string{"validate", scenario})
	if code != 2 {
		t.Fatalf("expected exit code 2 for deny, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "Rule IDs:") {
		t.Fatalf("expected Rule IDs section, got: %s", out)
	}
	if !strings.Contains(out, "How to fix:") {
		t.Fatalf("expected How to fix section, got: %s", out)
	}
}

func TestValidateFailJSONOutput(t *testing.T) {
	scenario := filepath.Join("examples", "terraform_public_exposure_fail.json")
	code, out, errOut := runFromRepoRoot(t, []string{"validate", "--json", scenario})
	if code != 2 {
		t.Fatalf("expected exit code 2 for deny, got %d stderr=%s", code, errOut)
	}
	var resp struct {
		Status   string   `json:"status"`
		RuleIDs  []string `json:"rule_ids"`
		Reason   string   `json:"reason"`
		Hints    []string `json:"hints"`
		Evidence string   `json:"evidence_id"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if resp.Status != "FAIL" {
		t.Fatalf("expected FAIL status, got %s", resp.Status)
	}
	if len(resp.RuleIDs) == 0 {
		t.Fatalf("expected rule IDs, got none")
	}
	if len(resp.Hints) == 0 {
		t.Fatalf("expected hints, got none")
	}
	if resp.Evidence == "" {
		t.Fatalf("expected evidence id, got empty")
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

func TestHelpMentionsDefaultEvidenceStore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("EVIDRA_HOME", home)
	t.Setenv("EVIDRA_EVIDENCE_DIR", "")
	t.Setenv("EVIDRA_EVIDENCE_PATH", "")

	code, _, errOut := runFromRepoRoot(t, []string{"--help"})
	if code != 2 {
		t.Fatalf("expected code 2 for help, got %d", code)
	}
	wantPath := filepath.Join(home, filepath.FromSlash(config.DefaultEvidenceRelativeDir))
	if !strings.Contains(errOut, wantPath) {
		t.Fatalf("expected help to mention %q, got: %s", wantPath, errOut)
	}
}

func TestValidateHelpMentionsDefaultEvidenceStore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("EVIDRA_HOME", home)
	t.Setenv("EVIDRA_EVIDENCE_DIR", "")
	t.Setenv("EVIDRA_EVIDENCE_PATH", "")

	code, _, errOut := runFromRepoRoot(t, []string{"validate", "--help"})
	if code != 2 {
		t.Fatalf("expected code 2 for help, got %d", code)
	}
	wantPath := filepath.Join(home, filepath.FromSlash(config.DefaultEvidenceRelativeDir))
	if !strings.Contains(errOut, wantPath) {
		t.Fatalf("expected validate help to mention %q, got: %s", wantPath, errOut)
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
