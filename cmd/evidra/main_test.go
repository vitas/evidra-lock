package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"samebits.com/evidra/pkg/config"
	"samebits.com/evidra/pkg/scenario"
)

func TestValidateUsage(t *testing.T) {
	code, _, stderr := runFromRepoRoot(t, []string{"validate"})
	if code != 4 {
		t.Fatalf("expected exit code 4 for missing file, got %d", code)
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
		Source   string   `json:"source"`
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
	if resp.Source != "local" {
		t.Fatalf("expected source=local, got %s", resp.Source)
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
	if code != 4 {
		t.Fatalf("expected code 4 for help, got %d", code)
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
	if code != 4 {
		t.Fatalf("expected code 4 for help, got %d", code)
	}
	// Validate help no longer shows default evidence path (it's in usage text)
	if !strings.Contains(errOut, "usage: evidra validate") {
		t.Fatalf("expected validate usage, got: %s", errOut)
	}
}

// --- Hybrid mode tests ---

func TestValidate_OnlineMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/validate" {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"allow":       true,
			"risk_level":  "low",
			"evidence_id": "evt-online-1",
			"policy_ref":  "ops-v0.1",
		})
	}))
	defer srv.Close()

	t.Setenv("EVIDRA_URL", srv.URL)
	t.Setenv("EVIDRA_API_KEY", "test-key-for-online-validation")

	scenario := filepath.Join("examples", "terraform_plan_pass.json")
	code, out, errOut := runFromRepoRoot(t, []string{"validate", "--json", scenario})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut)
	}
	var resp validationJSON
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("parse JSON: %v\nraw: %s", err, out)
	}
	if resp.Source != "api" {
		t.Errorf("expected source=api, got %s", resp.Source)
	}
	if resp.Mode != "online" {
		t.Errorf("expected mode=online, got %s", resp.Mode)
	}
}

func TestValidate_OfflineFlag(t *testing.T) {
	t.Setenv("EVIDRA_URL", "https://should-not-be-reached.example.com")
	t.Setenv("EVIDRA_API_KEY", "test-key-for-offline-flag")

	scenario := filepath.Join("examples", "terraform_plan_pass.json")
	code, out, errOut := runFromRepoRoot(t, []string{"validate", "--offline", "--json", scenario})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut)
	}
	var resp validationJSON
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if resp.Source != "local" {
		t.Errorf("expected source=local with --offline, got %s", resp.Source)
	}
	if resp.Mode != "offline" {
		t.Errorf("expected mode=offline, got %s", resp.Mode)
	}
}

func TestValidate_FallbackClosed(t *testing.T) {
	// Get a dead port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	t.Setenv("EVIDRA_URL", "http://"+addr)
	t.Setenv("EVIDRA_API_KEY", "test-key-for-fallback-closed")

	scenario := filepath.Join("examples", "terraform_plan_pass.json")
	code, _, errOut := runFromRepoRoot(t, []string{"validate", scenario})
	if code != 3 {
		t.Fatalf("expected exit code 3 for unreachable API, got %d stderr=%s", code, errOut)
	}
	if !strings.Contains(errOut, "API unreachable") {
		t.Fatalf("expected unreachable message, got: %s", errOut)
	}
	if !strings.Contains(errOut, "EVIDRA_FALLBACK=offline") {
		t.Fatalf("expected hint about EVIDRA_FALLBACK, got: %s", errOut)
	}
}

func TestValidate_FallbackClosed_JSON(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	t.Setenv("EVIDRA_URL", "http://"+addr)
	t.Setenv("EVIDRA_API_KEY", "test-key-for-fallback-closed-json")

	scenario := filepath.Join("examples", "terraform_plan_pass.json")
	code, out, _ := runFromRepoRoot(t, []string{"validate", "--json", scenario})
	if code != 3 {
		t.Fatalf("expected exit code 3, got %d", code)
	}
	var resp validationJSON
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("parse JSON: %v\nraw: %s", err, out)
	}
	if resp.Error == nil {
		t.Fatal("expected error in JSON")
	}
	if resp.Error.Code != "API_UNREACHABLE" {
		t.Errorf("expected error.code=API_UNREACHABLE, got %s", resp.Error.Code)
	}
	if resp.Mode != "online" {
		t.Errorf("expected mode=online, got %s", resp.Mode)
	}
	if resp.Error.URL == "" {
		t.Error("expected error.url to be set")
	}
}

func TestValidate_FallbackOffline(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	t.Setenv("EVIDRA_URL", "http://"+addr)
	t.Setenv("EVIDRA_API_KEY", "test-key-for-fallback-offline")
	t.Setenv("EVIDRA_FALLBACK", "offline")

	scenario := filepath.Join("examples", "terraform_plan_pass.json")
	code, out, errOut := runFromRepoRoot(t, []string{"validate", "--json", scenario})
	if code != 0 {
		t.Fatalf("expected exit code 0 with fallback, got %d stderr=%s", code, errOut)
	}
	var resp validationJSON
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if resp.Source != "local-fallback" {
		t.Errorf("expected source=local-fallback, got %s", resp.Source)
	}
	if !strings.Contains(errOut, "falling back to local") {
		t.Errorf("expected fallback message in stderr, got: %s", errOut)
	}
}

func TestValidate_AuthError_NoFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer srv.Close()

	t.Setenv("EVIDRA_URL", srv.URL)
	t.Setenv("EVIDRA_API_KEY", "bad-key")
	t.Setenv("EVIDRA_FALLBACK", "offline")

	scenario := filepath.Join("examples", "terraform_plan_pass.json")
	code, _, _ := runFromRepoRoot(t, []string{"validate", scenario})
	if code != 1 {
		t.Fatalf("expected exit code 1 for auth error, got %d", code)
	}
}

func TestValidate_RateLimit_NoFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer srv.Close()

	t.Setenv("EVIDRA_URL", srv.URL)
	t.Setenv("EVIDRA_API_KEY", "test-key-for-rate-limit")
	t.Setenv("EVIDRA_FALLBACK", "offline")

	scenario := filepath.Join("examples", "terraform_plan_pass.json")
	code, _, _ := runFromRepoRoot(t, []string{"validate", scenario})
	// Rate limit returns exit code 1 (not 3, even with fallback=offline)
	if code != 1 {
		t.Fatalf("expected exit code 1 for rate limit, got %d", code)
	}
}

func TestValidate_Denied_ExitCode2(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"allow":      false,
			"risk_level": "high",
			"reasons":    []string{"protected namespace"},
			"rule_ids":   []string{"k8s.protected_namespace"},
		})
	}))
	defer srv.Close()

	t.Setenv("EVIDRA_URL", srv.URL)
	t.Setenv("EVIDRA_API_KEY", "test-key-for-denied")

	scenario := filepath.Join("examples", "terraform_plan_pass.json")
	code, _, _ := runFromRepoRoot(t, []string{"validate", scenario})
	if code != 2 {
		t.Fatalf("expected exit code 2 for policy deny, got %d", code)
	}
}

func TestValidate_UsageError_ExitCode4(t *testing.T) {
	code, _, _ := runFromRepoRoot(t, []string{"validate"})
	if code != 4 {
		t.Fatalf("expected exit code 4 for usage error, got %d", code)
	}
}

func TestValidate_FallbackOfflineFlag(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	t.Setenv("EVIDRA_URL", "http://"+addr)
	t.Setenv("EVIDRA_API_KEY", "test-key-for-fallback-flag")

	scenario := filepath.Join("examples", "terraform_plan_pass.json")
	code, out, _ := runFromRepoRoot(t, []string{"validate", "--fallback-offline", "--json", scenario})
	if code != 0 {
		t.Fatalf("expected exit code 0 with --fallback-offline, got %d", code)
	}
	var resp validationJSON
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if resp.Source != "local-fallback" {
		t.Errorf("expected source=local-fallback, got %s", resp.Source)
	}
}

func TestValidate_EnvNormalization(t *testing.T) {
	var receivedEnv string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var inv struct {
			Environment string `json:"environment"`
		}
		json.NewDecoder(r.Body).Decode(&inv)
		receivedEnv = inv.Environment
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"allow":      true,
			"risk_level": "low",
		})
	}))
	defer srv.Close()

	t.Setenv("EVIDRA_URL", srv.URL)
	t.Setenv("EVIDRA_API_KEY", "test-key-for-env-norm")
	t.Setenv("EVIDRA_ENVIRONMENT", "prod")

	scenario := filepath.Join("examples", "terraform_plan_pass.json")
	code, _, errOut := runFromRepoRoot(t, []string{"validate", scenario})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut)
	}
	if receivedEnv != "production" {
		t.Errorf("expected environment=production, got %s", receivedEnv)
	}
}

func TestValidate_TimeoutFlag(t *testing.T) {
	// Server that never responds — the client timeout should trigger
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	// Accept connections but never respond
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// Hold connection open indefinitely
			_ = conn
		}
	}()

	t.Setenv("EVIDRA_URL", "http://"+ln.Addr().String())
	t.Setenv("EVIDRA_API_KEY", "test-key-for-timeout")

	sc := filepath.Join("examples", "terraform_plan_pass.json")
	code, _, _ := runFromRepoRoot(t, []string{"validate", "--timeout", "100ms", sc})
	if code != 3 {
		t.Fatalf("expected exit code 3 for timeout, got %d", code)
	}
}

func TestValidate_InvalidTimeout(t *testing.T) {
	scenario := filepath.Join("examples", "terraform_plan_pass.json")
	code, _, stderr := runFromRepoRoot(t, []string{"validate", "--timeout", "invalid", scenario})
	if code != 4 {
		t.Fatalf("expected exit code 4 for invalid timeout, got %d stderr=%s", code, stderr)
	}
}

// --- actionToInvocation contract test ---

func TestActionToInvocation_KeysMatchAllowlist(t *testing.T) {
	t.Parallel()
	sc := scenario.Scenario{
		ScenarioID: "test-1",
		Actor:      scenario.Actor{Type: "agent", ID: "test", Origin: "cli"},
		Source:     "cli",
	}
	action := scenario.Action{
		Kind:     "kubectl.apply",
		Target:   map[string]interface{}{"namespace": "default"},
		Payload:  map[string]interface{}{"manifest": "pod.yaml"},
		RiskTags: []string{"prod"},
		Intent:   "deploy",
	}
	inv := actionToInvocation(sc, action, "production")

	// Validate that the invocation passes structure validation
	err := inv.ValidateStructure()
	if err != nil {
		t.Fatalf("actionToInvocation produced invalid ToolInvocation: %v", err)
	}
}

func TestActionToInvocation_RoundTrip(t *testing.T) {
	t.Parallel()
	sc := scenario.Scenario{
		ScenarioID: "sc-123",
		Actor:      scenario.Actor{Type: "human", ID: "user1", Origin: "cli"},
		Source:     "cli",
	}
	action := scenario.Action{
		Kind:   "terraform.plan",
		Target: map[string]interface{}{"region": "us-east-1"},
	}
	inv := actionToInvocation(sc, action, "staging")

	if inv.Tool != "terraform" {
		t.Errorf("expected tool=terraform, got %s", inv.Tool)
	}
	if inv.Operation != "plan" {
		t.Errorf("expected operation=plan, got %s", inv.Operation)
	}
	if inv.Environment != "staging" {
		t.Errorf("expected environment=staging, got %s", inv.Environment)
	}
	if inv.Actor.ID != "user1" {
		t.Errorf("expected actor.id=user1, got %s", inv.Actor.ID)
	}
}

// --- Helpers ---

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
