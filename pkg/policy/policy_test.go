package policy

import (
	"path/filepath"
	"testing"

	"samebits.com/evidra-mcp/pkg/invocation"
)

func TestEvaluateDefaultDeny(t *testing.T) {
	policyPath, err := filepath.Abs(filepath.Join("..", "..", "policy", "policy.rego"))
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}

	engine, err := LoadFromFile(policyPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
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
