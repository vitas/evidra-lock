package policy

import (
	"path/filepath"
	"strings"
	"testing"

	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/policysource"
)

func TestEvaluateDefaultDecisionWithoutActions(t *testing.T) {
	policyPath := filepath.Join("..", "..", "policy", "profiles", "ops-v0.1", "policy.rego")
	dataPath := filepath.Join("..", "..", "policy", "profiles", "ops-v0.1", "data.json")

	src := policysource.NewLocalFileSource(policyPath, dataPath)
	policyModules, err := src.LoadPolicy()
	if err != nil {
		t.Fatalf("LoadPolicy() error = %v", err)
	}
	for key := range policyModules {
		if strings.Contains(key, "/tests/") {
			delete(policyModules, key)
		}
	}
	dataBytes, err := src.LoadData()
	if err != nil {
		t.Fatalf("LoadData() error = %v", err)
	}

	engine, err := NewOPAEngine(policyModules, dataBytes)
	if err != nil {
		t.Fatalf("NewOPAEngine() error = %v", err)
	}

	decision, err := engine.Evaluate(invocation.ToolInvocation{
		Actor: invocation.Actor{
			Type:   "human",
			ID:     "u1",
			Origin: "cli",
		},
		Tool:      "unknown",
		Operation: "run",
		Params:    map[string]interface{}{},
		Context:   map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !decision.Allow {
		t.Fatalf("expected default allow decision when no actions present, got deny")
	}
	if decision.Reason != "ok" {
		t.Fatalf("expected reason ok for base decision, got %q", decision.Reason)
	}
	if decision.RiskLevel != "normal" {
		t.Fatalf("expected normal risk_level by default, got %q", decision.RiskLevel)
	}
}

func TestEvaluateSupportsRichDecisionFields(t *testing.T) {
	policyBytes := []byte(`
package evidra.policy

import rego.v1

decision := {
  "allow": false,
  "risk_level": "high",
  "reason": "policy_denied_default",
  "reasons": ["policy_denied_default", "secondary_reason"],
  "hints": ["first_hint", "second_hint"],
  "hits": ["deny.default", "deny.secondary"],
}
`)

	engine, err := NewOPAEngine(map[string][]byte{"policy.rego": policyBytes}, nil)
	if err != nil {
		t.Fatalf("NewOPAEngine() error = %v", err)
	}
	decision, err := engine.Evaluate(invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "human", ID: "u1", Origin: "cli"},
		Tool:      "x",
		Operation: "y",
		Params:    map[string]interface{}{},
		Context:   map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if decision.Reason != "policy_denied_default" {
		t.Fatalf("unexpected reason %q", decision.Reason)
	}
	if len(decision.Reasons) != 2 || decision.Reasons[0] != "policy_denied_default" {
		t.Fatalf("unexpected reasons: %+v", decision.Reasons)
	}
	if len(decision.Hints) != 2 || decision.Hints[0] != "first_hint" {
		t.Fatalf("unexpected hints: %+v", decision.Hints)
	}
	if len(decision.Hits) != 2 || decision.Hits[0] != "deny.default" {
		t.Fatalf("unexpected hits: %+v", decision.Hits)
	}
}
