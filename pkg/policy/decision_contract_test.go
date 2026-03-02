package policy

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"samebits.com/evidra/pkg/bundlesource"
	"samebits.com/evidra/pkg/invocation"
)

func TestEngineEvaluateDecisionJSONContract_DenyVectors(t *testing.T) {
	t.Parallel()

	engine := newOpsBundleEngine(t)
	decision, err := engine.Evaluate(denyContractInvocation())
	if err != nil {
		t.Fatalf("Evaluate() error: %v", err)
	}
	if decision.Reason == "" {
		t.Fatal("expected non-empty reason")
	}
	if !isValidRiskLevel(decision.RiskLevel) {
		t.Fatalf("unexpected risk level %q", decision.RiskLevel)
	}
	if len(decision.Reasons) == 0 {
		t.Fatal("expected reasons to be populated")
	}
	if len(decision.Hits) == 0 {
		t.Fatal("expected hits to be populated")
	}
	if len(decision.Hints) == 0 {
		t.Fatal("expected hints to be populated")
	}

	m := marshalDecisionToMap(t, decision)
	assertJSONHasKeys(t, m, "allow", "risk_level", "reason", "reasons", "hits", "hints")
	assertJSONBool(t, m, "allow")
	assertJSONString(t, m, "risk_level")
	assertJSONString(t, m, "reason")
	assertJSONStringSlice(t, m, "reasons")
	assertJSONStringSlice(t, m, "hits")
	assertJSONStringSlice(t, m, "hints")
}

func TestEngineEvaluateDecisionJSONContract_AllowCoreFields(t *testing.T) {
	t.Parallel()

	engine := newOpsBundleEngine(t)
	decision, err := engine.Evaluate(allowContractInvocation())
	if err != nil {
		t.Fatalf("Evaluate() error: %v", err)
	}
	if decision.Reason == "" {
		t.Fatal("expected non-empty reason")
	}
	if !isValidRiskLevel(decision.RiskLevel) {
		t.Fatalf("unexpected risk level %q", decision.RiskLevel)
	}
	if len(decision.Reasons) == 0 {
		t.Fatal("expected reasons to be populated")
	}

	m := marshalDecisionToMap(t, decision)
	assertJSONHasKeys(t, m, "allow", "risk_level", "reason", "reasons")
	assertJSONBool(t, m, "allow")
	assertJSONString(t, m, "risk_level")
	assertJSONString(t, m, "reason")
	assertJSONStringSlice(t, m, "reasons")

	// hits/hints are optional vectors for non-triggering inputs; if present, keep types stable.
	if _, ok := m["hits"]; ok {
		assertJSONStringSlice(t, m, "hits")
	}
	if _, ok := m["hints"]; ok {
		assertJSONStringSlice(t, m, "hints")
	}
}

func newOpsBundleEngine(t *testing.T) *Engine {
	t.Helper()

	repoRoot, err := repoRootFromCaller()
	if err != nil {
		t.Fatal(err)
	}
	bundleDir := filepath.Join(repoRoot, "policy", "bundles", "ops-v0.1")
	src, err := bundlesource.NewBundleSource(bundleDir)
	if err != nil {
		t.Fatalf("NewBundleSource() error: %v", err)
	}
	modules, err := src.LoadPolicy()
	if err != nil {
		t.Fatalf("LoadPolicy() error: %v", err)
	}
	dataBytes, err := src.LoadData()
	if err != nil {
		t.Fatalf("LoadData() error: %v", err)
	}
	engine, err := NewOPAEngine(modules, dataBytes)
	if err != nil {
		t.Fatalf("NewOPAEngine() error: %v", err)
	}
	return engine
}

func denyContractInvocation() invocation.ToolInvocation {
	return invocation.ToolInvocation{
		Actor:       invocation.Actor{Type: "human", ID: "decision-contract-test", Origin: "cli"},
		Tool:        "kubectl",
		Operation:   "delete",
		Environment: "prod",
		Params: map[string]interface{}{
			"action": map[string]interface{}{
				"kind":      "kubectl.delete",
				"risk_tags": []interface{}{},
				"target": map[string]interface{}{
					"namespace": "prod",
				},
				"payload": map[string]interface{}{
					"namespace":      "prod",
					"resource_count": 7,
				},
			},
		},
	}
}

func allowContractInvocation() invocation.ToolInvocation {
	return invocation.ToolInvocation{
		Actor:       invocation.Actor{Type: "human", ID: "decision-contract-test", Origin: "cli"},
		Tool:        "kubectl",
		Operation:   "apply",
		Environment: "dev",
		Params: map[string]interface{}{
			"action": map[string]interface{}{
				"kind":      "kubectl.apply",
				"risk_tags": []interface{}{},
				"target": map[string]interface{}{
					"namespace": "default",
				},
				"payload": map[string]interface{}{
					"namespace": "default",
					"resource":  "configmap",
				},
			},
		},
	}
}

func marshalDecisionToMap(t *testing.T, decision Decision) map[string]interface{} {
	t.Helper()

	raw, err := json.Marshal(decision)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	return m
}

func assertJSONHasKeys(t *testing.T, m map[string]interface{}, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if _, ok := m[key]; !ok {
			t.Fatalf("JSON output missing key %q; got keys: %v", key, mapKeys(m))
		}
	}
}

func assertJSONBool(t *testing.T, m map[string]interface{}, key string) {
	t.Helper()
	v, ok := m[key].(bool)
	if !ok {
		t.Fatalf("expected %q to be bool, got %T", key, m[key])
	}
	_ = v
}

func assertJSONString(t *testing.T, m map[string]interface{}, key string) {
	t.Helper()
	v, ok := m[key].(string)
	if !ok {
		t.Fatalf("expected %q to be string, got %T", key, m[key])
	}
	if v == "" {
		t.Fatalf("expected %q to be non-empty", key)
	}
}

func assertJSONStringSlice(t *testing.T, m map[string]interface{}, key string) {
	t.Helper()
	arr, ok := m[key].([]interface{})
	if !ok {
		t.Fatalf("expected %q to be []interface{}, got %T", key, m[key])
	}
	for i, item := range arr {
		if _, ok := item.(string); !ok {
			t.Fatalf("expected %q[%d] to be string, got %T", key, i, item)
		}
	}
}

func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
