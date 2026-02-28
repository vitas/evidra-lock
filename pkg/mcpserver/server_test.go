package mcpserver

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra/pkg/invocation"
)

var bundleDir = filepath.Join("..", "..", "policy", "bundles", "ops-v0.1")

func TestValidateServiceReturnsDeny(t *testing.T) {
	opts := Options{
		BundlePath:   bundleDir,
		EvidencePath: t.TempDir(),
		Mode:         ModeEnforce,
	}
	svc := newValidateService(opts)

	inv := invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "human", ID: "tester", Origin: "cli"},
		Tool:      "kubectl",
		Operation: "delete",
		Params: map[string]interface{}{
			"payload": map[string]interface{}{
				"namespace": "prod",
			},
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
		if id == "ops.unapproved_change" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing ops.unapproved_change in %v", out.RuleIDs)
	}
	if len(out.Hints) == 0 {
		t.Fatalf("expected hints")
	}
}

func TestValidateServiceRecordsEvidence(t *testing.T) {
	opts := Options{
		BundlePath:   bundleDir,
		EvidencePath: t.TempDir(),
		Mode:         ModeEnforce,
	}
	svc := newValidateService(opts)

	inv := invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "human", ID: "tester", Origin: "cli"},
		Tool:      "kubectl",
		Operation: "delete",
		Params: map[string]interface{}{
			"target":  map[string]interface{}{"namespace": "default"},
			"payload": map[string]interface{}{"namespace": "default", "resource": "pod"},
		},
		Context: map[string]interface{}{},
	}

	out := svc.Validate(context.Background(), inv)
	if !out.Policy.Allow {
		t.Fatalf("expected policy to allow; rule_ids=%v reasons=%v", out.RuleIDs, out.Reasons)
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

func TestServerRegistersValidateTool(t *testing.T) {
	server := newTestServer(t)
	tools := listToolNamesFromServer(t, server)
	if !containsTool(tools, "validate") {
		t.Fatalf("expected validate tool in %v", tools)
	}
	if containsTool(tools, "execute") {
		t.Fatalf("did not expect execute tool by default")
	}
}

func newTestServer(t *testing.T) *mcp.Server {
	t.Helper()
	opts := Options{
		BundlePath:   bundleDir,
		EvidencePath: t.TempDir(),
		Mode:         ModeEnforce,
	}
	return NewServer(opts)
}

func listToolNamesFromServer(t *testing.T, srv *mcp.Server) []string {
	t.Helper()
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer clientSession.Close()
	res, err := clientSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools RPC: %v", err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	return names
}

func containsTool(list []string, name string) bool {
	for _, tool := range list {
		if tool == name {
			return true
		}
	}
	return false
}

func TestValidateServiceBadPolicyReturnsCode(t *testing.T) {
	svc := newValidateService(Options{
		PolicyPath:   "nonexistent.rego",
		DataPath:     "nonexistent.json",
		EvidencePath: t.TempDir(),
		Mode:         ModeEnforce,
	})
	inv := invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "human", ID: "u1", Origin: "test"},
		Tool:      "kubectl",
		Operation: "apply",
		Params:    map[string]interface{}{},
		Context:   map[string]interface{}{},
	}
	out := svc.Validate(context.Background(), inv)
	if out.OK {
		t.Fatal("expected OK=false when policy load fails")
	}
	if out.Error == nil {
		t.Fatal("expected Error to be set")
	}
	if out.Error.Code != ErrCodePolicyFailure {
		t.Errorf("expected error code %q, got %q", ErrCodePolicyFailure, out.Error.Code)
	}
}

func TestValidateServiceInvalidInputReturnsCode(t *testing.T) {
	svc := newValidateService(Options{Mode: ModeEnforce})
	out := svc.Validate(context.Background(), invocation.ToolInvocation{})
	if out.OK {
		t.Fatal("expected OK=false for invalid input")
	}
	if out.Error == nil {
		t.Fatal("expected Error to be set")
	}
	if out.Error.Code != ErrCodeInvalidInput {
		t.Errorf("expected error code %q, got %q", ErrCodeInvalidInput, out.Error.Code)
	}
}
