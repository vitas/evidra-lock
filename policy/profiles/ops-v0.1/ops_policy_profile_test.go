package opsprofile_test

import (
	"path/filepath"
	"strings"
	"testing"

	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/policy"
	"samebits.com/evidra-mcp/pkg/policysource"
)

func TestOpsPolicyKitDecisions(t *testing.T) {
	policyPath := filepath.Join("policy.rego")
	dataPath := filepath.Join("data.json")
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

	engine, err := policy.NewOPAEngine(policyModules, dataBytes)
	if err != nil {
		t.Fatalf("NewOPAEngine() error = %v", err)
	}

	cases := []struct {
		name   string
		inv    invocation.ToolInvocation
		allow  bool
		risk   string
		reason string
	}{
		{
			name:   "helm list in dev allowed low",
			inv:    inv("helm", "list", map[string]any{}, "dev"),
			allow:  true,
			risk:   "low",
			reason: "allowed_read_operation",
		},
		{
			name:   "helm upgrade in dev allowed high",
			inv:    inv("helm", "upgrade", map[string]any{"release": "app", "chart": "./chart", "namespace": "default"}, "dev"),
			allow:  true,
			risk:   "high",
			reason: "allowed_write_dev",
		},
		{
			name:   "terraform apply in prod allowed critical",
			inv:    inv("terraform", "apply", map[string]any{"dir": "./infra"}, "prod"),
			allow:  true,
			risk:   "critical",
			reason: "allowed_write_prod",
		},
		{
			name:   "aws recursive delete in prod denied critical",
			inv:    inv("aws", "s3-rm-recursive", map[string]any{"uri": "s3://my-bucket/tmp/"}, "prod"),
			allow:  false,
			risk:   "critical",
			reason: "policy_denied_high_risk",
		},
		{
			name:   "unknown tool denied critical",
			inv:    inv("unknown", "run", map[string]any{}, "dev"),
			allow:  false,
			risk:   "critical",
			reason: "policy_denied_default",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			d, err := engine.Evaluate(tc.inv)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if d.Allow != tc.allow {
				t.Fatalf("allow mismatch: got %v want %v", d.Allow, tc.allow)
			}
			if d.RiskLevel != tc.risk {
				t.Fatalf("risk_level mismatch: got %q want %q", d.RiskLevel, tc.risk)
			}
			if d.Reason != tc.reason {
				t.Fatalf("reason mismatch: got %q want %q", d.Reason, tc.reason)
			}
		})
	}
}

func inv(tool, op string, params map[string]any, env string) invocation.ToolInvocation {
	return invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "human", ID: "test", Origin: "unit"},
		Tool:      tool,
		Operation: op,
		Params:    params,
		Context: map[string]any{
			"environment": env,
		},
	}
}
