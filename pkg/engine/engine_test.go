package engine

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/policy"
)

func TestEnforceModePolicyDenyBlocksExecution(t *testing.T) {
	var executed int32
	resolver := &fakeResolver{
		tool: &fakeTool{
			name: "mock", op: "run",
			exec: func(ctx context.Context, params map[string]interface{}) (ExecutionOutput, error) {
				atomic.AddInt32(&executed, 1)
				code := 0
				return ExecutionOutput{Status: "success", ExitCode: &code, Stdout: "ok"}, nil
			},
		},
	}
	store := &memoryEvidenceStore{}
	eng := NewExecutionEngine(resolver, denyPolicyEngine{}, store, Config{Mode: ModeEnforce, PolicyRef: "p1"})

	res, err := eng.Execute(context.Background(), baseInvocation("mock", "run"), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.OK {
		t.Fatalf("expected ok=false")
	}
	if res.Output.Status != "denied" {
		t.Fatalf("expected denied status, got %q", res.Output.Status)
	}
	if atomic.LoadInt32(&executed) != 0 {
		t.Fatalf("expected executor not called")
	}
}

func TestObserveModePolicyDenyAllowsExecutionAdvisory(t *testing.T) {
	var executed int32
	resolver := &fakeResolver{
		tool: &fakeTool{
			name: "mock", op: "run",
			exec: func(ctx context.Context, params map[string]interface{}) (ExecutionOutput, error) {
				atomic.AddInt32(&executed, 1)
				code := 0
				return ExecutionOutput{Status: "success", ExitCode: &code, Stdout: "ok"}, nil
			},
		},
	}
	store := &memoryEvidenceStore{}
	eng := NewExecutionEngine(resolver, denyPolicyEngine{}, store, Config{Mode: ModeObserve, PolicyRef: "p1"})

	res, err := eng.Execute(context.Background(), baseInvocation("mock", "run"), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !res.OK {
		t.Fatalf("expected ok=true, got %+v", res.Error)
	}
	if atomic.LoadInt32(&executed) != 1 {
		t.Fatalf("expected executor called once")
	}
	if !res.Advisory {
		t.Fatalf("expected advisory=true")
	}
	if len(store.records) != 1 || !store.records[0].PolicyDecision.Advisory {
		t.Fatalf("expected advisory evidence record")
	}
}

func TestPolicyEvalFailureUsesGenericHint(t *testing.T) {
	resolver := &fakeResolver{tool: &fakeTool{name: "mock", op: "run", exec: successExec}}
	store := &memoryEvidenceStore{}
	eng := NewExecutionEngine(resolver, errorPolicyEngine{}, store, Config{Mode: ModeEnforce, PolicyRef: "p1"})

	res, err := eng.Execute(context.Background(), baseInvocation("mock", "run"), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.OK {
		t.Fatalf("expected deny on policy eval failure")
	}
	if res.Error == nil || res.Error.Code != "policy_evaluation_failed" {
		t.Fatalf("expected policy_evaluation_failed, got %+v", res.Error)
	}
	if res.Error.Hint == "" {
		t.Fatalf("expected generic hint")
	}
}

func TestEvidenceErrorMapping(t *testing.T) {
	tests := []struct {
		name      string
		appendErr error
		wantCode  string
	}{
		{
			name: "busy",
			appendErr: &evidence.StoreError{
				Code:    evidence.ErrorCodeStoreBusy,
				Message: "busy",
			},
			wantCode: evidence.ErrorCodeStoreBusy,
		},
		{name: "chain invalid", appendErr: evidence.ErrChainInvalid, wantCode: "evidence_chain_invalid"},
		{name: "internal", appendErr: errors.New("write failed"), wantCode: "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &fakeResolver{tool: &fakeTool{name: "mock", op: "run", exec: successExec}}
			store := &memoryEvidenceStore{appendErr: tt.appendErr}
			eng := NewExecutionEngine(resolver, allowPolicyEngine{}, store, Config{Mode: ModeEnforce, PolicyRef: "p1"})
			res, err := eng.Execute(context.Background(), baseInvocation("mock", "run"), nil)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if res.OK {
				t.Fatalf("expected ok=false")
			}
			if res.Error == nil || res.Error.Code != tt.wantCode {
				t.Fatalf("expected code %q, got %+v", tt.wantCode, res.Error)
			}
		})
	}
}

func TestValidatorsAggregateRisk(t *testing.T) {
	resolver := &fakeResolver{tool: &fakeTool{name: "mock", op: "run", exec: successExec}}
	store := &memoryEvidenceStore{}
	eng := NewExecutionEngine(resolver, allowPolicyEngine{}, store, Config{
		Mode:      ModeEnforce,
		PolicyRef: "p1",
		Validators: []Validator{
			fakeValidator{name: "v1", hits: []ValidationHit{{Severity: "medium", Message: "m1"}}},
			fakeValidator{name: "v2", hits: []ValidationHit{{Severity: "critical", Message: "c1"}}},
		},
	})

	res, err := eng.Execute(context.Background(), baseInvocation("mock", "run"), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !res.OK {
		t.Fatalf("expected ok=true")
	}
	if res.Decision.RiskLevel != "critical" {
		t.Fatalf("expected aggregated critical risk, got %q", res.Decision.RiskLevel)
	}
	if len(res.Hits) != 2 {
		t.Fatalf("expected 2 validator hits, got %d", len(res.Hits))
	}
}

func TestNoValidatorsPreservesBehavior(t *testing.T) {
	resolver := &fakeResolver{tool: &fakeTool{name: "mock", op: "run", exec: successExec}}
	store := &memoryEvidenceStore{}
	eng := NewExecutionEngine(resolver, allowPolicyEngine{}, store, Config{Mode: ModeEnforce, PolicyRef: "p1"})
	res, err := eng.Execute(context.Background(), baseInvocation("mock", "run"), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !res.OK {
		t.Fatalf("expected ok=true")
	}
	if res.Decision.RiskLevel != "low" {
		t.Fatalf("expected policy risk level low, got %q", res.Decision.RiskLevel)
	}
}

func successExec(ctx context.Context, params map[string]interface{}) (ExecutionOutput, error) {
	code := 0
	return ExecutionOutput{Status: "success", ExitCode: &code, Stdout: "ok"}, nil
}

func baseInvocation(tool, op string) invocation.ToolInvocation {
	return invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "human", ID: "u1", Origin: "mcp"},
		Tool:      tool,
		Operation: op,
		Params:    map[string]interface{}{},
		Context:   map[string]interface{}{},
	}
}

type fakeResolver struct {
	tool ToolDefinition
	err  error
}

func (r *fakeResolver) Resolve(tool string, op string) (ToolDefinition, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.tool, nil
}

type fakeTool struct {
	name string
	op   string
	meta ToolMetadata
	exec func(context.Context, map[string]interface{}) (ExecutionOutput, error)
}

func (t *fakeTool) Name() string                                  { return t.name }
func (t *fakeTool) Operation() string                             { return t.op }
func (t *fakeTool) ValidateParams(params map[string]string) error { return nil }
func (t *fakeTool) BuildCommand(params map[string]string) ([]string, error) {
	return nil, errors.New("unsupported")
}
func (t *fakeTool) Metadata() ToolMetadata                                { return t.meta }
func (t *fakeTool) ValidateRawParams(params map[string]interface{}) error { return nil }
func (t *fakeTool) Execute(ctx context.Context, params map[string]interface{}) (ExecutionOutput, error) {
	return t.exec(ctx, params)
}

type fakeValidator struct {
	name string
	hits []ValidationHit
	err  error
}

func (v fakeValidator) Name() string { return v.name }
func (v fakeValidator) Validate(ctx context.Context, inv invocation.ToolInvocation, tool ToolDefinition) ([]ValidationHit, error) {
	if v.err != nil {
		return nil, v.err
	}
	return v.hits, nil
}

type allowPolicyEngine struct{}

func (allowPolicyEngine) Evaluate(inv invocation.ToolInvocation) (policy.Decision, error) {
	return policy.Decision{Allow: true, RiskLevel: "low", Reason: "allowed_by_rule"}, nil
}

type denyPolicyEngine struct{}

func (denyPolicyEngine) Evaluate(inv invocation.ToolInvocation) (policy.Decision, error) {
	return policy.Decision{
		Allow:     false,
		RiskLevel: "critical",
		Reason:    "policy_denied_default",
		Hints:     []string{"use approved context"},
		Hint:      "use approved context",
	}, nil
}

type errorPolicyEngine struct{}

func (errorPolicyEngine) Evaluate(inv invocation.ToolInvocation) (policy.Decision, error) {
	return policy.Decision{}, errors.New("policy engine failed")
}

type memoryEvidenceStore struct {
	records   []evidence.Record
	appendErr error
}

func (m *memoryEvidenceStore) Append(rec evidence.Record) error {
	if m.appendErr != nil {
		return m.appendErr
	}
	m.records = append(m.records, rec)
	return nil
}

func (m *memoryEvidenceStore) ValidateChain() error      { return nil }
func (m *memoryEvidenceStore) LastHash() (string, error) { return "", nil }
