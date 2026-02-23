package mcpserver

import (
	"context"
	"path/filepath"
	"testing"

	"samebits.com/evidra-mcp/pkg/invocation"
)

func TestValidateServiceReturnsDeny(t *testing.T) {
	profileDir := filepath.Join("..", "..", "policy", "profiles", "ops-v0.1")
	opts := Options{
		PolicyPath:   filepath.Join(profileDir, "policy.rego"),
		DataPath:     filepath.Join(profileDir, "data.json"),
		EvidencePath: t.TempDir(),
		Mode:         ModeEnforce,
	}
	svc := newValidateService(opts)

	inv := invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "human", ID: "tester", Origin: "cli"},
		Tool:      "kubectl",
		Operation: "delete",
		Params: map[string]interface{}{
			"namespace": "prod",
		},
		Context: map[string]interface{}{},
	}

	out := svc.Validate(context.Background(), inv)
	if out.Policy.Allow {
		t.Fatalf("expected policy to deny")
	}
	if out.EventID == "" {
		t.Fatalf("missing event id")
	}
	if len(out.RuleIDs) == 0 {
		t.Fatalf("expected rule ids")
	}
	found := false
	for _, id := range out.RuleIDs {
		if id == "POL-PROD-01" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing POL-PROD-01 in %v", out.RuleIDs)
	}
	if len(out.Hints) == 0 {
		t.Fatalf("expected hints")
	}
}

func TestValidateServiceRecordsEvidence(t *testing.T) {
	profileDir := filepath.Join("..", "..", "policy", "profiles", "ops-v0.1")
	opts := Options{
		PolicyPath:   filepath.Join(profileDir, "policy.rego"),
		DataPath:     filepath.Join(profileDir, "data.json"),
		EvidencePath: t.TempDir(),
		Mode:         ModeEnforce,
	}
	svc := newValidateService(opts)

	inv := invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "human", ID: "tester", Origin: "cli"},
		Tool:      "kubectl",
		Operation: "delete",
		Params:    map[string]interface{}{},
		Context:   map[string]interface{}{},
	}

	out := svc.Validate(context.Background(), inv)
	if !out.Policy.Allow {
		t.Fatalf("expected policy to allow")
	}
	if out.EventID == "" {
		t.Fatalf("missing event id")
	}
	event := svc.GetEvent(context.Background(), out.EventID)
	if !event.OK {
		t.Fatalf("get event failed: %+v", event.Error)
	}
	if event.Record == nil {
		t.Fatalf("expected record")
	}
}
