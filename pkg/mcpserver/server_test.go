package mcpserver

import (
	"context"
	"encoding/json"
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

func TestExecuteAllowedStructuredResponse(t *testing.T) {
	svc := newService(t)
	out := svc.Execute(context.Background(), baseInvocation("echo", "run", map[string]interface{}{"text": "hello"}))

	if !out.OK {
		t.Fatalf("expected ok=true, got false: %+v", out)
	}
	if out.EventID == "" {
		t.Fatalf("expected event_id")
	}
	if out.Policy.Reason == "" || out.Policy.RiskLevel == "" {
		t.Fatalf("expected policy summary in response")
	}
	if out.Execution.Status != "success" {
		t.Fatalf("expected success status, got %q", out.Execution.Status)
	}
	if len(out.Resources) == 0 || out.Resources[0].URI == "" {
		t.Fatalf("expected resource links in response")
	}
	if lines := countEvidenceLines(t); lines != 1 {
		t.Fatalf("expected 1 evidence record, got %d", lines)
	}
}

func TestExecuteDeniedStructuredResponse(t *testing.T) {
	svc := newService(t)
	out := svc.Execute(context.Background(), baseInvocation("unknown", "run", map[string]interface{}{}))

	if out.OK {
		t.Fatalf("expected ok=false for denied response")
	}
	if out.EventID == "" {
		t.Fatalf("expected denial to include event_id")
	}
	if out.Error == nil {
		t.Fatalf("expected structured error")
	}
	if out.Error.Code != "unregistered_tool" {
		t.Fatalf("expected unregistered_tool code, got %q", out.Error.Code)
	}
	if out.Policy.PolicyRef == "" {
		t.Fatalf("expected policy_ref in response")
	}
	if out.Execution.Status != "denied" {
		t.Fatalf("expected denied status, got %q", out.Execution.Status)
	}
}

func TestGetEventWrappedResponses(t *testing.T) {
	svc := newService(t)
	out := svc.Execute(context.Background(), baseInvocation("echo", "run", map[string]interface{}{"text": "one"}))

	got := svc.GetEvent(context.Background(), out.EventID)
	if !got.OK || got.Record == nil {
		t.Fatalf("expected wrapped ok response with record, got %+v", got)
	}
	if got.Record.EventID != out.EventID {
		t.Fatalf("expected matching event_id, got %q", got.Record.EventID)
	}
	if len(got.Resources) == 0 {
		t.Fatalf("expected resource links in get_event response")
	}

	notFound := svc.GetEvent(context.Background(), "evt-missing")
	if notFound.OK {
		t.Fatalf("expected not found response")
	}
	if notFound.Error == nil || notFound.Error.Code != "not_found" {
		t.Fatalf("expected not_found error, got %+v", notFound)
	}
}

func TestExecuteProgressReporterStages(t *testing.T) {
	svc := newService(t)
	var seen []string
	reporter := func(_ float64, msg string) {
		seen = append(seen, msg)
	}

	out := svc.ExecuteWithReporter(context.Background(), baseInvocation("echo", "run", map[string]interface{}{"text": "hello"}), reporter)
	if !out.OK {
		t.Fatalf("expected execute success, got %+v", out)
	}
	mustContain := []string{
		"received",
		"validated invocation",
		"registry ok",
		"policy evaluated (allow/deny)",
		"execution started",
		"execution finished (writing evidence)",
		"done",
	}
	for _, m := range mustContain {
		found := false
		for _, got := range seen {
			if got == m {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected progress stage %q in %v", m, seen)
		}
	}
}

func TestExecuteWithoutProgressReporterStillWorks(t *testing.T) {
	svc := newService(t)
	out := svc.ExecuteWithReporter(context.Background(), baseInvocation("echo", "run", map[string]interface{}{"text": "hello"}), nil)
	if !out.OK {
		t.Fatalf("expected execute to work without reporter, got %+v", out)
	}
}

func TestFileResourceLinksDisabledByDefaultAndOptIn(t *testing.T) {
	svc := newService(t)
	links := svc.resourceLinks("evt-1")
	for _, l := range links {
		if strings.HasPrefix(l.URI, "file://") {
			t.Fatalf("file link should be disabled by default")
		}
	}

	svc.includeFileResourceLinks = true
	links = svc.resourceLinks("evt-1")
	foundFile := false
	for _, l := range links {
		if strings.HasPrefix(l.URI, "file://") {
			foundFile = true
			break
		}
	}
	if !foundFile {
		t.Fatalf("expected file:// resource link when includeFileResourceLinks is enabled")
	}
}

func TestExecuteOutputTruncationStoredInEvidence(t *testing.T) {
	temp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(temp); err != nil {
		t.Fatalf("Chdir(temp) error = %v", err)
	}

	store := evidence.NewStore()
	if err := store.Init(); err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	reg := registry.NewInMemoryRegistry([]registry.ToolDefinition{
		{
			Name:                "mock",
			SupportedOperations: []string{"run"},
			InputSchema:         "{}",
			ValidateParams:      func(_ string, _ map[string]interface{}) error { return nil },
			Executor: func(_ context.Context, _ registry.ToolInvocationInput) (registry.ExecutionResult, error) {
				code := 0
				return registry.ExecutionResult{
					Status:   "success",
					ExitCode: &code,
					Stdout:   strings.Repeat("A", 200),
					Stderr:   strings.Repeat("B", 200),
				}, nil
			},
		},
	})

	svc := NewExecuteServiceWithMode(reg, allowPolicyEngine{}, store, ModeEnforce, "test-policy-ref")
	svc.maxOutputBytes = 32
	out := svc.Execute(context.Background(), baseInvocation("mock", "run", map[string]interface{}{}))
	if !out.Execution.StdoutTruncated || !out.Execution.StderrTruncated {
		t.Fatalf("expected truncation flags in response: %+v", out.Execution)
	}
	if !strings.Contains(out.Execution.Stdout, "[truncated]") || !strings.Contains(out.Execution.Stderr, "[truncated]") {
		t.Fatalf("expected truncation marker in response")
	}

	records, err := evidence.ReadAllAtPath(filepath.Join("data", "evidence"))
	if err != nil {
		t.Fatalf("ReadAllAtPath() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 evidence record, got %d", len(records))
	}
	rec := records[0]
	if !rec.ExecutionResult.StdoutTruncated || !rec.ExecutionResult.StderrTruncated {
		t.Fatalf("expected truncation flags in evidence record: %+v", rec.ExecutionResult)
	}
	if rec.ExecutionResult.Stdout != out.Execution.Stdout || rec.ExecutionResult.Stderr != out.Execution.Stderr {
		t.Fatalf("expected evidence output to match response output")
	}
}

func TestGetEventFailsOnInvalidChain(t *testing.T) {
	svc := newService(t)
	out := svc.Execute(context.Background(), baseInvocation("echo", "run", map[string]interface{}{"text": "one"}))
	_ = svc.Execute(context.Background(), baseInvocation("echo", "run", map[string]interface{}{"text": "two"}))

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

	resp := svc.GetEvent(context.Background(), out.EventID)
	if resp.OK {
		t.Fatalf("expected chain invalid response")
	}
	if resp.Error == nil || resp.Error.Code != "evidence_chain_invalid" {
		t.Fatalf("expected evidence_chain_invalid, got %+v", resp)
	}
}

func TestToolMetadataIncludesDescriptionsAndAnnotations(t *testing.T) {
	temp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(temp); err != nil {
		t.Fatalf("Chdir(temp) error = %v", err)
	}

	policyPath, err := policyPathFromWorkingDir(oldWd)
	if err != nil {
		t.Fatalf("policyPathFromWorkingDir() error = %v", err)
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
	server := NewServer(Options{Name: "evidra-mcp-test", Version: "v0.1.0", PolicyRef: "test-policy-ref"}, registry.NewDefaultRegistry(), policyEngine, store)

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

	tools, err := clientSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(tools.Tools) == 0 {
		t.Fatalf("expected at least one tool")
	}

	foundExecute := false
	foundGetEvent := false
	for _, tool := range tools.Tools {
		switch tool.Name {
		case "execute":
			foundExecute = true
			if tool.Description == "" {
				t.Fatalf("execute description missing")
			}
			if tool.Annotations == nil {
				t.Fatalf("execute annotations missing")
			}
		case "get_event":
			foundGetEvent = true
			if tool.Description == "" {
				t.Fatalf("get_event description missing")
			}
			if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
				t.Fatalf("get_event read-only annotation missing")
			}
		}
	}
	if !foundExecute || !foundGetEvent {
		t.Fatalf("expected execute and get_event tools in metadata")
	}
}

func TestMCPExecuteAndGetEventStructuredOutputs(t *testing.T) {
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
		t.Fatalf("policyPathFromWorkingDir() error = %v", err)
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
	server := NewServer(Options{Name: "evidra-mcp-test", Version: "v0.1.0", PolicyRef: "test-policy-ref"}, registry.NewDefaultRegistry(), policyEngine, store)

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

	execRes, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "execute",
		Arguments: map[string]any{
			"actor":     map[string]any{"type": "human", "id": "u1", "origin": "mcp"},
			"tool":      "echo",
			"operation": "run",
			"params":    map[string]any{"text": "hello"},
			"context":   map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("execute call error = %v", err)
	}
	if execRes.IsError {
		t.Fatalf("execute call should return structured output, not MCP error")
	}
	execOut, ok := execRes.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected execute structured content object")
	}
	if okVal, _ := execOut["ok"].(bool); !okVal {
		t.Fatalf("expected execute ok=true, got %v", execOut["ok"])
	}
	eventID, _ := execOut["event_id"].(string)
	if eventID == "" {
		t.Fatalf("expected event_id in execute response")
	}
	if len(execRes.Content) == 0 {
		t.Fatalf("expected resource link content in execute call result")
	}

	getRes, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_event",
		Arguments: map[string]any{"event_id": eventID},
	})
	if err != nil {
		t.Fatalf("get_event call error = %v", err)
	}
	if getRes.IsError {
		t.Fatalf("get_event should return structured response")
	}
	getOut, ok := getRes.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected get_event structured content object")
	}
	if okVal, _ := getOut["ok"].(bool); !okVal {
		t.Fatalf("expected get_event ok=true, got %v", getOut["ok"])
	}
	if _, exists := getOut["record"]; !exists {
		t.Fatalf("expected record wrapper in get_event response")
	}

	readRes, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "evidra://event/" + eventID,
	})
	if err != nil {
		t.Fatalf("ReadResource(event) error = %v", err)
	}
	if len(readRes.Contents) == 0 || readRes.Contents[0].Text == "" {
		t.Fatalf("expected event resource content")
	}
	var eventRecord map[string]any
	if err := json.Unmarshal([]byte(readRes.Contents[0].Text), &eventRecord); err != nil {
		t.Fatalf("unmarshal event resource content: %v", err)
	}
	if gotID, _ := eventRecord["event_id"].(string); gotID != eventID {
		t.Fatalf("expected event_id %q in resource content, got %q", eventID, gotID)
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

	return NewExecuteServiceWithMode(registry.NewDefaultRegistry(), policyEngine, store, mode, "test-policy-ref")
}

func policyPathFromWorkingDir(wd string) (string, error) {
	return filepath.Abs(filepath.Join(wd, "..", "..", "policy", "policy.rego"))
}

func baseInvocation(tool, operation string, params map[string]interface{}) invocation.ToolInvocation {
	return invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "human", ID: "u1", Origin: "mcp"},
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

type allowPolicyEngine struct{}

func (allowPolicyEngine) Evaluate(inv invocation.ToolInvocation) (policy.Decision, error) {
	return policy.Decision{
		Allow:     true,
		RiskLevel: "low",
		Reason:    "allowed_by_rule",
	}, nil
}
