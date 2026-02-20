package policy

import (
	"os"
	"path/filepath"
	"testing"

	"samebits.com/evidra-mcp/pkg/invocation"
)

func TestAWSS3BasicPolicy(t *testing.T) {
	root := filepath.Join("..", "..")
	policyPath := filepath.Join(root, "packs", "_core", "ops", "aws-s3-basic", "policy", "policy.rego")
	dataPath := filepath.Join(root, "packs", "_core", "ops", "aws-s3-basic", "policy", "data.example.json")

	policyBytes, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("ReadFile(policy.rego) error = %v", err)
	}
	dataBytes, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("ReadFile(data.example.json) error = %v", err)
	}

	engine, err := NewOPAEngine(policyBytes, dataBytes)
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
			name:  "s3 ls allowed low",
			inv:   awsInv("s3-ls", map[string]any{"uri": "s3://my-bucket/tmp/"}, "dev"),
			allow: true, risk: "low", reason: "allowed_read_operation",
		},
		{
			name:  "s3 rm object dev allowed for allowlist prefix",
			inv:   awsInv("s3-rm-object", map[string]any{"uri": "s3://my-bucket/tmp/file.txt"}, "dev"),
			allow: true, risk: "high", reason: "allowed_s3_delete_object_dev",
		},
		{
			name:  "s3 rm object denied when prefix not allowlisted",
			inv:   awsInv("s3-rm-object", map[string]any{"uri": "s3://other-bucket/file.txt"}, "dev"),
			allow: false, risk: "critical", reason: "policy_denied_high_risk",
		},
		{
			name:  "s3 rm recursive dev allowed only on allowlist",
			inv:   awsInv("s3-rm-recursive", map[string]any{"uri": "s3://my-bucket/scratch/"}, "dev"),
			allow: true, risk: "critical", reason: "allowed_s3_delete_recursive_dev",
		},
		{
			name:  "s3 rm recursive prod denied",
			inv:   awsInv("s3-rm-recursive", map[string]any{"uri": "s3://my-bucket/tmp/"}, "prod"),
			allow: false, risk: "critical", reason: "denied_s3_delete_recursive_prod",
		},
		{
			name:  "s3 uri validation requires s3 prefix",
			inv:   awsInv("s3-ls", map[string]any{"uri": "https://bucket/key"}, "dev"),
			allow: false, risk: "critical", reason: "policy_denied_default",
		},
		{
			name:  "sts whoami not allowed in ops pack policy",
			inv:   awsInv("sts-whoami", map[string]any{}, "dev"),
			allow: false, risk: "critical", reason: "policy_denied_default",
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
				t.Fatalf("risk mismatch: got %q want %q", d.RiskLevel, tc.risk)
			}
			if d.Reason != tc.reason {
				t.Fatalf("reason mismatch: got %q want %q", d.Reason, tc.reason)
			}
		})
	}
}

func awsInv(op string, params map[string]any, env string) invocation.ToolInvocation {
	return invocation.ToolInvocation{
		Actor: invocation.Actor{
			Type:   "human",
			ID:     "ops-user",
			Origin: "mcp",
		},
		Tool:      "aws",
		Operation: op,
		Params:    params,
		Context:   map[string]any{"environment": env},
	}
}
