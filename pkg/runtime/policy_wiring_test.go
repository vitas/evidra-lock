package runtime_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/policy"
	"samebits.com/evidra-mcp/pkg/policysource"
	"samebits.com/evidra-mcp/pkg/runtime"
)

// fakeSource is a test double for runtime.PolicySource.
type fakeSource struct {
	modules       map[string][]byte
	data          []byte
	ref           string
	loadPolicyErr error
	loadDataErr   error
	policyRefErr  error
}

func (f *fakeSource) LoadPolicy() (map[string][]byte, error) { return f.modules, f.loadPolicyErr }
func (f *fakeSource) LoadData() ([]byte, error)              { return f.data, f.loadDataErr }
func (f *fakeSource) PolicyRef() (string, error)             { return f.ref, f.policyRefErr }

var policyProfileDir = filepath.Join("..", "..", "policy", "profiles", "ops-v0.1")

func TestRuntimeEvaluatorPolicyWiring(t *testing.T) {
	eval := newPolicyEvaluator(t)

	denyAction := map[string]interface{}{
		"kind":      "kubectl.delete",
		"target":    "prod-delete",
		"risk_tags": []string{},
		"payload": map[string]interface{}{
			"namespace":      "prod",
			"resource_count": 7,
		},
	}
	decision := evaluateInvocation(t, eval, []map[string]interface{}{denyAction}, "prod", "delete")
	if decision.Allow {
		t.Fatalf("expected deny decision for prod delete action, got allow")
	}
	if decision.RiskLevel != "high" {
		t.Fatalf("expected risk_level high, got %q", decision.RiskLevel)
	}
	if decision.Reason == "" {
		t.Fatal("expected reason to be non-empty")
	}
	assertStringsContain(t, decision.Hits, "POL-PROD-01")
	assertNotEmpty(t, decision.Hints, "hints")
	assertNotEmpty(t, decision.Reasons, "reasons")

	safeAction := map[string]interface{}{
		"kind":      "kubectl.apply",
		"target":    "default-apply",
		"risk_tags": []string{},
		"payload": map[string]interface{}{
			"namespace": "default",
		},
	}
	allowed := evaluateInvocation(t, eval, []map[string]interface{}{safeAction}, "dev", "apply")
	if !allowed.Allow {
		t.Fatalf("expected allow decision for safe action, got deny")
	}
	if allowed.RiskLevel != "low" {
		t.Fatalf("expected risk_level low, got %q", allowed.RiskLevel)
	}
}

func newPolicyEvaluator(t *testing.T) *runtime.Evaluator {
	t.Helper()
	policyPath := filepath.Join(policyProfileDir, "policy.rego")
	dataPath := filepath.Join(policyProfileDir, "data.json")
	eval, err := runtime.NewEvaluator(policysource.NewLocalFileSource(policyPath, dataPath))
	if err != nil {
		t.Fatalf("NewEvaluator() error = %v", err)
	}
	return eval
}

func evaluateInvocation(t *testing.T, eval *runtime.Evaluator, actions []map[string]interface{}, env, operation string) policy.Decision {
	t.Helper()
	inv := invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "human", ID: "test-user", Origin: "runtime-test"},
		Tool:      "kubectl",
		Operation: operation,
		Params: map[string]interface{}{
			"actions": mapSliceToInterface(actions),
		},
		Context: map[string]interface{}{
			"environment": env,
		},
	}
	decision, err := eval.EvaluateInvocation(inv)
	if err != nil {
		t.Fatalf("EvaluateInvocation() error = %v", err)
	}
	return decision
}

func mapSliceToInterface(actions []map[string]interface{}) []interface{} {
	out := make([]interface{}, 0, len(actions))
	for _, act := range actions {
		out = append(out, act)
	}
	return out
}

func assertStringsContain(t *testing.T, slice []string, target string) {
	t.Helper()
	for _, v := range slice {
		if v == target {
			return
		}
	}
	t.Fatalf("expected %q in %v", target, slice)
}

func assertNotEmpty(t *testing.T, slice []string, name string) {
	t.Helper()
	if len(slice) == 0 {
		t.Fatalf("expected %s to be populated", name)
	}
}

func TestEvaluateInvocationSetsPolicyRef(t *testing.T) {
	eval := newPolicyEvaluator(t)
	action := map[string]interface{}{
		"kind":      "kubectl.apply",
		"risk_tags": []string{},
		"payload":   map[string]interface{}{"namespace": "default"},
	}
	decision := evaluateInvocation(t, eval, []map[string]interface{}{action}, "dev", "apply")
	if decision.PolicyRef == "" {
		t.Fatal("expected PolicyRef to be set on returned Decision, got empty string")
	}
}

func TestNewEvaluatorLoadPolicyError(t *testing.T) {
	src := &fakeSource{loadPolicyErr: errors.New("disk read failed")}
	_, err := runtime.NewEvaluator(src)
	if err == nil {
		t.Fatal("expected error when LoadPolicy fails, got nil")
	}
}

func TestNewEvaluatorLoadDataError(t *testing.T) {
	src := &fakeSource{
		modules:     map[string][]byte{"p.rego": []byte(`package p`)},
		loadDataErr: errors.New("data read failed"),
	}
	_, err := runtime.NewEvaluator(src)
	if err == nil {
		t.Fatal("expected error when LoadData fails, got nil")
	}
}

func TestNewEvaluatorPolicyRefError(t *testing.T) {
	// Use a real policy dir so OPA compilation succeeds; only PolicyRef fails.
	policyPath := filepath.Join(policyProfileDir, "policy.rego")
	dataPath := filepath.Join(policyProfileDir, "data.json")
	real := policysource.NewLocalFileSource(policyPath, dataPath)
	modules, err := real.LoadPolicy()
	if err != nil {
		t.Fatalf("setup: LoadPolicy: %v", err)
	}
	data, err := real.LoadData()
	if err != nil {
		t.Fatalf("setup: LoadData: %v", err)
	}
	src := &fakeSource{
		modules:      modules,
		data:         data,
		policyRefErr: errors.New("ref hash failed"),
	}
	_, err = runtime.NewEvaluator(src)
	if err == nil {
		t.Fatal("expected error when PolicyRef fails, got nil")
	}
}

func TestDecisionJSONShape(t *testing.T) {
	sd := policy.Decision{
		Allow:     true,
		RiskLevel: "medium",
		Reason:    "breakglass",
		PolicyRef: "sha256:abc123",
		Reasons:   []string{"breakglass"},
		Hints:     []string{"hint-1"},
		Hits:      []string{"WARN-BREAKGLASS-01"},
	}
	data, err := json.Marshal(sd)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	for _, key := range []string{"allow", "risk_level", "reason", "reasons", "hints", "hits", "policy_ref"} {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON output missing key %q; got keys: %v", key, mapKeys(m))
		}
	}
	if m["allow"] != true {
		t.Errorf("allow: got %v, want true", m["allow"])
	}
	if m["policy_ref"] != "sha256:abc123" {
		t.Errorf("policy_ref: got %v, want sha256:abc123", m["policy_ref"])
	}
}

func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
