package mcpserver

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/policy"
	"samebits.com/evidra-mcp/pkg/registry"
)

func TestRegistryDenyUnregisteredTool(t *testing.T) {
	svc := newService(t)
	out, err := svc.Execute(context.Background(), baseInvocation("unknown", "run", map[string]interface{}{}))
	if err == nil {
		t.Fatalf("expected deny error, got nil")
	}
	if out.Status != "denied" {
		t.Fatalf("expected denied status, got %q", out.Status)
	}
	if lines := countEvidenceLines(t); lines != 1 {
		t.Fatalf("expected 1 evidence record, got %d", lines)
	}
}

func TestRegistryDenyUnsupportedOperation(t *testing.T) {
	svc := newService(t)
	out, err := svc.Execute(context.Background(), baseInvocation("git", "push", map[string]interface{}{}))
	if err == nil {
		t.Fatalf("expected deny error, got nil")
	}
	if out.Status != "denied" {
		t.Fatalf("expected denied status, got %q", out.Status)
	}
	if lines := countEvidenceLines(t); lines != 1 {
		t.Fatalf("expected 1 evidence record, got %d", lines)
	}
}

func TestEvidenceWrittenOnDeny(t *testing.T) {
	svc := newService(t)
	_, _ = svc.Execute(context.Background(), baseInvocation("unknown", "run", map[string]interface{}{}))
	if lines := countEvidenceLines(t); lines != 1 {
		t.Fatalf("expected 1 evidence record, got %d", lines)
	}
	if err := evidence.ValidateChain(); err != nil {
		t.Fatalf("ValidateChain() error = %v", err)
	}
}

func TestEvidenceWrittenOnSuccess(t *testing.T) {
	svc := newService(t)
	out, err := svc.Execute(context.Background(), baseInvocation("echo", "run", map[string]interface{}{"text": "hello"}))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out.Status != "success" {
		t.Fatalf("expected success status, got %q", out.Status)
	}
	if lines := countEvidenceLines(t); lines != 1 {
		t.Fatalf("expected 1 evidence record, got %d", lines)
	}
	if err := evidence.ValidateChain(); err != nil {
		t.Fatalf("ValidateChain() error = %v", err)
	}
}

func TestMCPInvocationFlowAndEvidenceChain(t *testing.T) {
	temp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(temp); err != nil {
		t.Fatalf("Chdir(temp) error = %v", err)
	}

	repoPath := filepath.Join(temp, "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(repo) error = %v", err)
	}
	if out, err := exec.Command("git", "-C", repoPath, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init error = %v, out=%s", err, string(out))
	}

	policyPath, err := policyPathFromWorkingDir(oldWd)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	policyEngine, err := policy.LoadFromFile(policyPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}
	store := evidence.NewStore()
	if err := store.Init(); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	reg := registry.NewDefaultRegistry()
	server := NewServer(Options{Name: "evidra-mcp-test", Version: "v0.1.0"}, reg, policyEngine, store)

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	t.Cleanup(func() {
		serverSession.Close()
		serverSession.Wait()
	})

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.1.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	call := func(arguments map[string]any) (*mcp.CallToolResult, error) {
		return clientSession.CallTool(ctx, &mcp.CallToolParams{
			Name:      "execute",
			Arguments: arguments,
		})
	}

	echoRes, err := call(map[string]any{
		"actor":     map[string]any{"type": "human", "id": "u1", "origin": "mcp"},
		"tool":      "echo",
		"operation": "run",
		"params":    map[string]any{"text": "hello"},
		"context":   map[string]any{},
	})
	if err != nil {
		t.Fatalf("echo call error = %v", err)
	}
	if echoRes.IsError {
		t.Fatalf("echo call returned tool error")
	}

	gitRes, err := call(map[string]any{
		"actor":     map[string]any{"type": "human", "id": "u1", "origin": "mcp"},
		"tool":      "git",
		"operation": "status",
		"params":    map[string]any{"path": repoPath},
		"context":   map[string]any{},
	})
	if err != nil {
		t.Fatalf("git status call error = %v", err)
	}
	if gitRes.IsError {
		t.Fatalf("git status call returned tool error")
	}

	deniedRes, err := call(map[string]any{
		"actor":     map[string]any{"type": "human", "id": "u1", "origin": "mcp"},
		"tool":      "git",
		"operation": "push",
		"params":    map[string]any{},
		"context":   map[string]any{},
	})
	if err == nil && (deniedRes == nil || !deniedRes.IsError) {
		t.Fatalf("expected git push call to be denied")
	}

	if lines := countEvidenceLines(t); lines != 3 {
		t.Fatalf("expected 3 evidence records, got %d", lines)
	}
	if err := evidence.ValidateChain(); err != nil {
		t.Fatalf("ValidateChain() error = %v", err)
	}
}

func newService(t *testing.T) *ExecuteService {
	t.Helper()

	temp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(temp); err != nil {
		t.Fatalf("Chdir(temp) error = %v", err)
	}

	policyPath, err := policyPathFromWorkingDir(oldWd)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}

	policyEngine, err := policy.LoadFromFile(policyPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}
	store := evidence.NewStore()
	if err := store.Init(); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	return NewExecuteService(registry.NewDefaultRegistry(), policyEngine, store)
}

func policyPathFromWorkingDir(wd string) (string, error) {
	return filepath.Abs(filepath.Join(wd, "..", "..", "policy", "policy.rego"))
}

func baseInvocation(tool, operation string, params map[string]interface{}) invocation.ToolInvocation {
	return invocation.ToolInvocation{
		Actor: invocation.Actor{
			Type:   "human",
			ID:     "u1",
			Origin: "mcp",
		},
		Tool:      tool,
		Operation: operation,
		Params:    params,
		Context:   map[string]interface{}{},
	}
}

func countEvidenceLines(t *testing.T) int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("data", "evidence.log"))
	if err != nil {
		return 0
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "\n"))
}
