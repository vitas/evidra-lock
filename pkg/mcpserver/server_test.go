package mcpserver

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

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
	profileDir := filepath.Join("..", "..", "policy", "profiles", "ops-v0.1")
	opts := Options{
		PolicyPath:    filepath.Join(profileDir, "policy.rego"),
		DataPath:      filepath.Join(profileDir, "data.json"),
		EvidencePath:  t.TempDir(),
		Mode:          ModeEnforce,
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
