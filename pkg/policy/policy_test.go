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
