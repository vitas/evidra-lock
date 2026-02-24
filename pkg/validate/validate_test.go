package validate_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"samebits.com/evidra/pkg/invocation"
	"samebits.com/evidra/pkg/scenario"
	"samebits.com/evidra/pkg/validate"
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

// ---------------------------------------------------------------------------
// EvaluateScenario — policy outcome paths
// ---------------------------------------------------------------------------

func TestEvaluateScenario_Allow(t *testing.T) {
	result, err := validate.EvaluateScenario(context.Background(), safeScenario(), safeOpts(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Errorf("Pass=false, want true; reasons=%v", result.Reasons)
	}
	if result.RiskLevel != "low" {
		t.Errorf("RiskLevel=%q, want low", result.RiskLevel)
	}
	if result.EvidenceID == "" {
		t.Error("EvidenceID empty, want non-empty")
	}
}

func TestEvaluateScenario_Deny(t *testing.T) {
	// POL-PROD-01: prod namespace without change-approved tag.
	sc := scenario.Scenario{
		ScenarioID: "deny-test",
		Actor:      scenario.Actor{Type: "human", ID: "u1", Origin: "test"},
		Source:     "test",
		Timestamp:  time.Now().UTC(),
		Actions: []scenario.Action{
			{
				Kind:   "kubectl.delete",
				Target: map[string]interface{}{"namespace": "prod"},
				Payload: map[string]interface{}{
					"namespace":      "prod",
					"resource_count": 3,
				},
			},
		},
	}
	result, err := validate.EvaluateScenario(context.Background(), sc, safeOpts(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Error("Pass=true, want false")
	}
	if result.RiskLevel != "high" {
		t.Errorf("RiskLevel=%q, want high", result.RiskLevel)
	}
	if len(result.Reasons) == 0 {
		t.Error("Reasons empty, want at least one")
	}
	if !containsString(result.RuleIDs, "POL-PROD-01") {
		t.Errorf("RuleIDs=%v, want POL-PROD-01", result.RuleIDs)
	}
	if len(result.Hints) == 0 {
		t.Error("Hints empty, want at least one")
	}
}

func TestEvaluateScenario_WarnBreakglass(t *testing.T) {
	// WARN-BREAKGLASS-01: breakglass tag present → allowed, WARN rule fires.
	sc := scenario.Scenario{
		ScenarioID: "breakglass-test",
		Actor:      scenario.Actor{Type: "human", ID: "u1", Origin: "test"},
		Source:     "test",
		Timestamp:  time.Now().UTC(),
		Actions: []scenario.Action{
			{
				Kind:     "kubectl.apply",
				RiskTags: []string{"breakglass"},
				Target:   map[string]interface{}{"namespace": "default"},
				Payload:  map[string]interface{}{"namespace": "default"},
			},
		},
	}
	result, err := validate.EvaluateScenario(context.Background(), sc, safeOpts(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Errorf("Pass=false, want true; reasons=%v", result.Reasons)
	}
	if !containsString(result.RuleIDs, "WARN-BREAKGLASS-01") {
		t.Errorf("RuleIDs=%v, want WARN-BREAKGLASS-01", result.RuleIDs)
	}
	if len(result.Hints) == 0 {
		t.Error("Hints empty, want hint for WARN-BREAKGLASS-01")
	}
}

func TestEvaluateScenario_SkipEvidence(t *testing.T) {
	opts := safeOpts(t)
	opts.SkipEvidence = true
	result, err := validate.EvaluateScenario(context.Background(), safeScenario(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EvidenceID != "" {
		t.Errorf("EvidenceID=%q, want empty when SkipEvidence=true", result.EvidenceID)
	}
}

func TestEvaluateScenario_InvalidActionKind(t *testing.T) {
	// An action whose Kind has no dot separator fails splitKind.
	sc := scenario.Scenario{
		ScenarioID: "bad-kind",
		Actor:      scenario.Actor{Type: "human", ID: "u1", Origin: "test"},
		Source:     "test",
		Timestamp:  time.Now().UTC(),
		Actions: []scenario.Action{
			{Kind: "nodot"},
		},
	}
	result, err := validate.EvaluateScenario(context.Background(), sc, safeOpts(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Error("Pass=true, want false for invalid action kind")
	}
	if result.RiskLevel != "high" {
		t.Errorf("RiskLevel=%q, want high", result.RiskLevel)
	}
	if len(result.Reasons) == 0 {
		t.Error("Reasons empty, want reason describing invalid kind")
	}
}

func TestEvaluateScenario_MultiAction_OneDeny(t *testing.T) {
	// First action passes; second denies. Overall must fail.
	sc := scenario.Scenario{
		ScenarioID: "multi-action",
		Actor:      scenario.Actor{Type: "human", ID: "u1", Origin: "test"},
		Source:     "test",
		Timestamp:  time.Now().UTC(),
		Actions: []scenario.Action{
			{
				Kind:    "kubectl.apply",
				Target:  map[string]interface{}{"namespace": "default"},
				Payload: map[string]interface{}{"namespace": "default"},
			},
			{
				Kind:   "kubectl.delete",
				Target: map[string]interface{}{"namespace": "prod"},
				Payload: map[string]interface{}{
					"namespace":      "prod",
					"resource_count": 3,
				},
			},
		},
	}
	result, err := validate.EvaluateScenario(context.Background(), sc, safeOpts(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Error("Pass=true, want false when any action is denied")
	}
	if !containsString(result.RuleIDs, "POL-PROD-01") {
		t.Errorf("RuleIDs=%v, want POL-PROD-01 from denied action", result.RuleIDs)
	}
}

// ---------------------------------------------------------------------------
// EvaluateInvocation — field mapping reaches OPA
// ---------------------------------------------------------------------------

func TestEvaluateInvocation_PayloadReachesPolicy(t *testing.T) {
	// A prod-namespace payload in params["payload"] must reach OPA so that
	// POL-PROD-01 fires. This verifies invocationToScenario maps the payload
	// field correctly through the evaluation pipeline.
	inv := invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "human", ID: "u1", Origin: "test"},
		Tool:      "kubectl",
		Operation: "delete",
		Params: map[string]interface{}{
			"payload": map[string]interface{}{
				"namespace":      "prod",
				"resource_count": 3,
			},
		},
		Context: map[string]interface{}{},
	}
	result, err := validate.EvaluateInvocation(context.Background(), inv, safeOpts(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Error("Pass=true, want false — prod payload should trigger POL-PROD-01")
	}
	if !containsString(result.RuleIDs, "POL-PROD-01") {
		t.Errorf("RuleIDs=%v, want POL-PROD-01", result.RuleIDs)
	}
}

// ---------------------------------------------------------------------------
// Error sentinel tests (from TD-08)
// ---------------------------------------------------------------------------

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

func containsString(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
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
