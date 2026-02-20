package policy

import (
	"os"
	"path/filepath"
	"testing"

	"samebits.com/evidra-mcp/pkg/invocation"
)

func TestPodmanBasicPolicy(t *testing.T) {
	root := filepath.Join("..", "..")
	policyPath := filepath.Join(root, "packs", "_core", "ops", "podman-basic", "policy", "policy.rego")
	dataPath := filepath.Join(root, "packs", "_core", "ops", "podman-basic", "policy", "data.example.json")

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
			name:   "images allowed low",
			inv:    podmanInv("images", map[string]any{}, "dev"),
			allow:  true,
			risk:   "low",
			reason: "allowed_read_operation",
		},
		{
			name:   "push denied latest tag",
			inv:    podmanInv("push", map[string]any{"image": "registry.example.com/team/app:latest"}, "dev"),
			allow:  false,
			risk:   "critical",
			reason: "denied_push_disallowed_tag",
		},
		{
			name:   "push denied missing tag treated latest",
			inv:    podmanInv("push", map[string]any{"image": "registry.example.com/team/app"}, "dev"),
			allow:  false,
			risk:   "critical",
			reason: "denied_push_disallowed_tag",
		},
		{
			name:   "push denied unknown registry",
			inv:    podmanInv("push", map[string]any{"image": "docker.io/library/nginx:1.25.0"}, "dev"),
			allow:  false,
			risk:   "critical",
			reason: "denied_push_disallowed_registry",
		},
		{
			name:   "push allowed dev",
			inv:    podmanInv("push", map[string]any{"image": "registry.example.com/team/app:1.2.3"}, "dev"),
			allow:  true,
			risk:   "high",
			reason: "allowed_push_dev",
		},
		{
			name:   "push allowed prod critical",
			inv:    podmanInv("push", map[string]any{"image": "registry.example.com/team/app:1.2.3"}, "prod"),
			allow:  true,
			risk:   "critical",
			reason: "allowed_push_prod",
		},
		{
			name:   "tag denied disallowed registry",
			inv:    podmanInv("tag", map[string]any{"source": "myimg:1.2.3", "target": "docker.io/library/myimg:1.2.3"}, "dev"),
			allow:  false,
			risk:   "critical",
			reason: "denied_tag_disallowed_registry",
		},
		{
			name:   "tag denied disallowed tag",
			inv:    podmanInv("tag", map[string]any{"source": "myimg:1.2.3", "target": "registry.example.com/team/app:latest"}, "dev"),
			allow:  false,
			risk:   "critical",
			reason: "denied_tag_disallowed_tag",
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

func podmanInv(op string, params map[string]any, env string) invocation.ToolInvocation {
	return invocation.ToolInvocation{
		Actor: invocation.Actor{
			Type:   "human",
			ID:     "ops-user",
			Origin: "mcp",
		},
		Tool:      "podman",
		Operation: op,
		Params:    params,
		Context:   map[string]any{"environment": env},
	}
}
