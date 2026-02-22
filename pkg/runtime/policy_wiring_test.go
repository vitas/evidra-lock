package runtime_test

import (
	"path/filepath"
	"testing"

	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/runtime"
)

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
	if allowed.RiskLevel != "normal" {
		t.Fatalf("expected risk_level normal, got %q", allowed.RiskLevel)
	}
}

func newPolicyEvaluator(t *testing.T) *runtime.Evaluator {
	t.Helper()
	policyPath := filepath.Join(policyProfileDir, "policy.rego")
	dataPath := filepath.Join(policyProfileDir, "data.json")
	eval, err := runtime.NewEvaluator(policyPath, dataPath)
	if err != nil {
		t.Fatalf("NewEvaluator() error = %v", err)
	}
	return eval
}

func evaluateInvocation(t *testing.T, eval *runtime.Evaluator, actions []map[string]interface{}, env, operation string) runtime.ScenarioDecision {
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
