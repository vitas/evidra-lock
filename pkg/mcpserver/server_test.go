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
	policyBytes, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("ReadFile(policy.rego) error = %v", err)
	}
	policyEngine, err := policy.NewOPAEngine(policyBytes, nil)
	if err != nil {
		t.Fatalf("NewOPAEngine() error = %v", err)
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

func TestEnforceModePolicyDenyBlocksExecution(t *testing.T) {
	policyPath := writePolicyFile(t, `package evidra.policy
import rego.v1
decision := {"allow": false, "risk_level": "critical", "reason": "policy_denied_default"}
`)

	svc := newServiceWithModeAndPolicyPath(t, ModeEnforce, policyPath)
	out, err := svc.Execute(context.Background(), baseInvocation("echo", "run", map[string]interface{}{"text": "hello"}))
	if err == nil {
		t.Fatalf("expected policy deny error in enforce mode")
	}
	if out.Status != "denied" {
		t.Fatalf("expected denied status, got %q", out.Status)
	}

	rec := readLastEvidenceRecord(t)
	if rec.PolicyDecision.Advisory {
		t.Fatalf("expected advisory=false in enforce mode")
	}
}

func TestObserveModePolicyDenyExecutesAndMarksAdvisory(t *testing.T) {
	policyPath := writePolicyFile(t, `package evidra.policy
import rego.v1
decision := {"allow": false, "risk_level": "critical", "reason": "policy_denied_default"}
`)

	svc := newServiceWithModeAndPolicyPath(t, ModeObserve, policyPath)
	out, err := svc.Execute(context.Background(), baseInvocation("echo", "run", map[string]interface{}{"text": "hello"}))
	if err != nil {
		t.Fatalf("expected execution to proceed in observe mode, got error: %v", err)
	}
	if out.Status != "success" {
		t.Fatalf("expected success status, got %q", out.Status)
	}

	rec := readLastEvidenceRecord(t)
	if !rec.PolicyDecision.Advisory {
		t.Fatalf("expected advisory=true in observe mode")
	}
	if rec.PolicyDecision.Allow {
		t.Fatalf("expected recorded policy decision allow=false")
	}
}

func TestGetEventFound(t *testing.T) {
	svc := newService(t)
	out1, err := svc.Execute(context.Background(), baseInvocation("echo", "run", map[string]interface{}{"text": "one"}))
	if err != nil {
		t.Fatalf("Execute(one) error = %v", err)
	}
	_, err = svc.Execute(context.Background(), baseInvocation("echo", "run", map[string]interface{}{"text": "two"}))
	if err != nil {
		t.Fatalf("Execute(two) error = %v", err)
	}

	rec, err := svc.GetEvent(context.Background(), out1.EventID)
	if err != nil {
		t.Fatalf("GetEvent() error = %v", err)
	}
	if rec.EventID != out1.EventID {
		t.Fatalf("expected event_id %q, got %q", out1.EventID, rec.EventID)
	}
	if rec.Tool != "echo" || rec.Operation != "run" {
		t.Fatalf("unexpected record tool/operation: %s/%s", rec.Tool, rec.Operation)
	}
}

func TestGetEventNotFound(t *testing.T) {
	svc := newService(t)
	_, err := svc.Execute(context.Background(), baseInvocation("echo", "run", map[string]interface{}{"text": "one"}))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	rec, err := svc.GetEvent(context.Background(), "evt-missing")
	if err == nil {
		t.Fatalf("expected not found error")
	}
	if rec.EventID != "" {
		t.Fatalf("expected empty record on not found")
	}
}

func TestGetEventFailsOnInvalidChain(t *testing.T) {
	svc := newService(t)
	out, err := svc.Execute(context.Background(), baseInvocation("echo", "run", map[string]interface{}{"text": "one"}))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	_, err = svc.Execute(context.Background(), baseInvocation("echo", "run", map[string]interface{}{"text": "two"}))
	if err != nil {
		t.Fatalf("Execute() second error = %v", err)
	}

	path := filepath.Join("data", "evidence", "segments", "evidence-000001.jsonl")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(segment) error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 evidence lines")
	}
	lines[0] = strings.Replace(lines[0], "\"status\":\"success\"", "\"status\":\"tampered\"", 1)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(segment) error = %v", err)
	}

	rec, err := svc.GetEvent(context.Background(), out.EventID)
	if err == nil {
		t.Fatalf("expected chain invalid error")
	}
	if err.Error() != "evidence_chain_invalid" {
		t.Fatalf("expected evidence_chain_invalid error, got %v", err)
	}
	if rec.EventID != "" {
		t.Fatalf("expected empty record on chain invalid")
	}
}

func TestGetEventAcrossSegments(t *testing.T) {
	t.Setenv("EVIDRA_EVIDENCE_SEGMENT_MAX_BYTES", "400")
	svc := newService(t)

	first, err := svc.Execute(context.Background(), baseInvocation("echo", "run", map[string]interface{}{"text": strings.Repeat("a", 280)}))
	if err != nil {
		t.Fatalf("Execute(first) error = %v", err)
	}
	for i := 0; i < 4; i++ {
		if _, err := svc.Execute(context.Background(), baseInvocation("echo", "run", map[string]interface{}{"text": strings.Repeat("b", 280)})); err != nil {
			t.Fatalf("Execute(%d) error = %v", i, err)
		}
	}

	rec, err := svc.GetEvent(context.Background(), first.EventID)
	if err != nil {
		t.Fatalf("GetEvent() error = %v", err)
	}
	if rec.EventID != first.EventID {
		t.Fatalf("expected event_id %q, got %q", first.EventID, rec.EventID)
	}
}

func newService(t *testing.T) *ExecuteService {
	return newServiceWithMode(t, ModeEnforce)
}

func newServiceWithMode(t *testing.T, mode Mode) *ExecuteService {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	policyPath, err := policyPathFromWorkingDir(wd)
	if err != nil {
		t.Fatalf("policyPathFromWorkingDir() error = %v", err)
	}
	return newServiceWithModeAndPolicyPath(t, mode, policyPath)
}

func newServiceWithModeAndPolicyPath(t *testing.T, mode Mode, policyPath string) *ExecuteService {
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

	policyBytes, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("ReadFile(policy.rego) error = %v", err)
	}
	policyEngine, err := policy.NewOPAEngine(policyBytes, nil)
	if err != nil {
		t.Fatalf("NewOPAEngine() error = %v", err)
	}
	store := evidence.NewStore()
	if err := store.Init(); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}

	return NewExecuteServiceWithMode(registry.NewDefaultRegistry(), policyEngine, store, mode, "")
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
	records, err := evidence.ReadAllAtPath(filepath.Join("data", "evidence"))
	if err != nil {
		return 0
	}
	return len(records)
}

func readLastEvidenceRecord(t *testing.T) evidence.EvidenceRecord {
	t.Helper()
	records, err := evidence.ReadAllAtPath(filepath.Join("data", "evidence"))
	if err != nil {
		t.Fatalf("ReadAllAtPath(evidence) error = %v", err)
	}
	if len(records) == 0 {
		t.Fatalf("no evidence records found")
	}
	return records[len(records)-1]
}

func writePolicyFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.rego")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(policy.rego) error = %v", err)
	}
	return path
}
