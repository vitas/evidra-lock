package policy

import (
	"os"
	"path/filepath"
	"testing"

	"samebits.com/evidra-mcp/pkg/invocation"
)

func TestEvaluateDefaultDeny(t *testing.T) {
	policyPath, err := filepath.Abs(filepath.Join("..", "..", "policy", "profiles", "ops-v0.1", "policy.rego"))
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	dataPath, err := filepath.Abs(filepath.Join("..", "..", "policy", "profiles", "ops-v0.1", "data.json"))
	if err != nil {
		t.Fatalf("filepath.Abs(data) error = %v", err)
	}

	policyBytes, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("ReadFile(policy.rego) error = %v", err)
	}
	dataBytes, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("ReadFile(data.json) error = %v", err)
	}

	engine, err := NewOPAEngine(policyBytes, dataBytes)
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
	if decision.Allow {
		t.Fatalf("expected default deny decision, got allow=true")
	}
	if decision.Reason == "" {
		t.Fatalf("expected non-empty reason")
	}
	if decision.RiskLevel != "critical" {
		t.Fatalf("expected critical risk_level on default deny, got %q", decision.RiskLevel)
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

	engine, err := NewOPAEngine(policyBytes, nil)
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
