package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra-mcp/pkg/core"
	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/policy"
	"samebits.com/evidra-mcp/pkg/registry"
)

type Options struct {
	Name         string
	Version      string
	Mode         Mode
	PolicyRef    string
	EvidencePath string
}

type Mode string

const (
	ModeEnforce Mode = "enforce"
	ModeObserve Mode = "observe"
)

type ExecuteOutput struct {
	OK        bool             `json:"ok"`
	EventID   string           `json:"event_id,omitempty"`
	Policy    PolicySummary    `json:"policy"`
	Execution ExecutionSummary `json:"execution"`
	Hints     []string         `json:"hints,omitempty"`
	Error     *ErrorSummary    `json:"error,omitempty"`
}

type PolicySummary struct {
	Allow     bool   `json:"allow"`
	RiskLevel string `json:"risk_level"`
	Reason    string `json:"reason"`
	PolicyRef string `json:"policy_ref"`
}

type ExecutionSummary struct {
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
}

type ErrorSummary struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RiskLevel string `json:"risk_level,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Hint      string `json:"hint,omitempty"`
}

type GetEventOutput struct {
	OK     bool             `json:"ok"`
	Record *evidence.Record `json:"record,omitempty"`
	Error  *ErrorSummary    `json:"error,omitempty"`
}

type executeHandler struct {
	service *ExecuteService
}

type getEventHandler struct {
	service *ExecuteService
}

type getEventInput struct {
	EventID string `json:"event_id"`
}

func NewServer(opts Options, reg registry.Registry, policyEngine core.PolicyEngine, evidenceStore core.EvidenceStore) *mcp.Server {
	if opts.Name == "" {
		opts.Name = "evidra-mcp"
	}
	if opts.Version == "" {
		opts.Version = "v0.1.0"
	}
	if opts.Mode == "" {
		opts.Mode = ModeEnforce
	}
	if opts.EvidencePath == "" {
		opts.EvidencePath = "./data/evidence"
	}

	svc := NewExecuteServiceWithMode(reg, policyEngine, evidenceStore, opts.Mode, opts.PolicyRef)
	svc.evidencePath = opts.EvidencePath
	executeTool := &executeHandler{service: svc}
	getEventTool := &getEventHandler{service: svc}

	server := mcp.NewServer(
		&mcp.Implementation{Name: opts.Name, Version: opts.Version},
		nil,
	)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "execute",
		Title:       "Execute Tool Invocation",
		Description: "Execute a canonical ToolInvocation through Registry -> Policy -> Execution -> Evidence. Typical use: controlled ops actions with event_id evidence linkage.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Controlled Execution",
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			DestructiveHint: boolPtr(true),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"actor", "tool", "operation", "params", "context"},
			"properties": map[string]any{
				"actor": map[string]any{
					"type":        "object",
					"description": "Execution initiator identity.",
					"required":    []string{"type", "id", "origin"},
					"properties": map[string]any{
						"type":   map[string]any{"type": "string", "description": "Actor category (human|ai|system)."},
						"id":     map[string]any{"type": "string", "description": "Actor identifier."},
						"origin": map[string]any{"type": "string", "description": "Invocation source (mcp|cli|api|unknown)."},
					},
				},
				"tool":      map[string]any{"type": "string", "description": "Registered tool name (required). Example: terraform, argocd, helm."},
				"operation": map[string]any{"type": "string", "description": "Registered operation for tool (required). Example: plan, app-sync, upgrade."},
				"params":    map[string]any{"type": "object", "description": "Operation parameters; validated by tool schema."},
				"context":   map[string]any{"type": "object", "description": "Execution context for policy decisions. Example: {\"environment\":\"dev\"}."},
			},
		},
	}, executeTool.Handle)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_event",
		Title:       "Get Evidence Event",
		Description: "Fetch one immutable evidence record by event_id. Typical use: inspect policy decision and execution details after execute.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Evidence Lookup",
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"event_id"},
			"properties": map[string]any{
				"event_id": map[string]any{"type": "string", "description": "Evidence event identifier from execute response (required). Example: evt-1234567890."},
			},
		},
	}, getEventTool.Handle)
	return server
}

func (h *executeHandler) Handle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input invocation.ToolInvocation,
) (*mcp.CallToolResult, ExecuteOutput, error) {
	output := h.service.Execute(ctx, input)
	return nil, output, nil
}

func (h *getEventHandler) Handle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input getEventInput,
) (*mcp.CallToolResult, GetEventOutput, error) {
	output := h.service.GetEvent(ctx, input.EventID)
	return nil, output, nil
}

type ExecuteService struct {
	registry     registry.Registry
	policy       core.PolicyEngine
	evidence     core.EvidenceStore
	mode         Mode
	policyRef    string
	evidencePath string
}

func NewExecuteService(reg registry.Registry, policyEngine core.PolicyEngine, evidenceStore core.EvidenceStore) *ExecuteService {
	return NewExecuteServiceWithMode(reg, policyEngine, evidenceStore, ModeEnforce, "")
}

func NewExecuteServiceWithMode(reg registry.Registry, policyEngine core.PolicyEngine, evidenceStore core.EvidenceStore, mode Mode, policyRef string) *ExecuteService {
	if mode == "" {
		mode = ModeEnforce
	}
	return &ExecuteService{
		registry:     reg,
		policy:       policyEngine,
		evidence:     evidenceStore,
		mode:         mode,
		policyRef:    policyRef,
		evidencePath: "./data/evidence",
	}
}

func (s *ExecuteService) GetEvent(_ context.Context, eventID string) GetEventOutput {
	if eventID == "" {
		return GetEventOutput{
			OK: false,
			Error: &ErrorSummary{
				Code:    "invalid_input",
				Message: "event_id is required",
			},
		}
	}

	rec, found, err := evidence.FindByEventID(s.evidencePath, eventID)
	if err != nil {
		if errors.Is(err, evidence.ErrChainInvalid) {
			return GetEventOutput{
				OK: false,
				Error: &ErrorSummary{
					Code:    "evidence_chain_invalid",
					Message: "evidence chain validation failed",
				},
			}
		}
		return GetEventOutput{
			OK: false,
			Error: &ErrorSummary{
				Code:    "internal_error",
				Message: "failed to read evidence",
			},
		}
	}
	if !found {
		return GetEventOutput{
			OK: false,
			Error: &ErrorSummary{
				Code:    "not_found",
				Message: "event_id not found",
			},
		}
	}
	return GetEventOutput{OK: true, Record: &rec}
}

func (s *ExecuteService) Execute(ctx context.Context, inv invocation.ToolInvocation) ExecuteOutput {
	if err := inv.ValidateStructure(); err != nil {
		return s.denyWithEvidence(inv, "invalid_invocation", err.Error(), "Provide actor/tool/operation and non-nil params/context.")
	}

	def, ok := s.registry.Lookup(inv.Tool)
	if !ok {
		return s.denyWithEvidence(inv, "unregistered_tool", fmt.Sprintf("tool %q is not registered", inv.Tool), "Install/enable the corresponding tool pack.")
	}
	if !registry.SupportsOperation(def, inv.Operation) {
		return s.denyWithEvidence(inv, "unsupported_operation", fmt.Sprintf("operation %q is not supported for tool %q", inv.Operation, inv.Tool), "Check supported operations for the registered tool.")
	}
	if err := registry.ValidateParams(def, inv.Operation, inv.Params); err != nil {
		return s.denyWithEvidence(inv, "invalid_params", err.Error(), "Fix params to match the operation schema.")
	}

	decision, evalErr := s.policy.Evaluate(inv)
	if evalErr != nil {
		decision = decisionForPolicyError()
	}
	if s.mode == ModeEnforce && !decision.Allow {
		hint := "Adjust policy or invocation context (e.g. context.environment), then re-run."
		if decision.Reason == "policy_evaluation_failed" {
			hint = "Check policy syntax and run evidra-policy-sim."
		}
		return s.writeFinal(inv, decision, registry.ExecutionResult{Status: "denied", ExitCode: nil}, false, &ErrorSummary{
			Code:      decision.Reason,
			Message:   "execution denied by policy",
			RiskLevel: decision.RiskLevel,
			Reason:    decision.Reason,
			Hint:      hint,
		}, false)
	}

	advisory := s.mode == ModeObserve
	execResult, execErr := def.Executor(ctx, registry.ToolInvocationInput{
		Operation: inv.Operation,
		Params:    inv.Params,
	})
	if execErr != nil {
		execResult.Status = "failed"
		if execResult.Stderr == "" {
			execResult.Stderr = execErr.Error()
		}
		return s.writeFinal(inv, decision, execResult, true, nil, advisory)
	}

	out := s.writeFinal(inv, decision, execResult, true, nil, advisory)
	if advisory && !decision.Allow {
		out.Hints = append(out.Hints, "observe mode: policy denied but execution was allowed")
	}
	return out
}

func (s *ExecuteService) denyWithEvidence(inv invocation.ToolInvocation, reason, msg, hint string) ExecuteOutput {
	return s.writeFinal(
		inv,
		decisionForDeny(reason),
		registry.ExecutionResult{Status: "denied", ExitCode: nil},
		false,
		&ErrorSummary{
			Code:      reason,
			Message:   msg,
			RiskLevel: "critical",
			Reason:    reason,
			Hint:      hint,
		},
		false,
	)
}

func (s *ExecuteService) writeFinal(
	inv invocation.ToolInvocation,
	decision policy.Decision,
	result registry.ExecutionResult,
	ok bool,
	errOut *ErrorSummary,
	advisory bool,
) ExecuteOutput {
	record := evidence.EvidenceRecord{
		EventID:   fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Timestamp: time.Now().UTC(),
		PolicyRef: s.policyRef,
		Actor:     inv.Actor,
		Tool:      inv.Tool,
		Operation: inv.Operation,
		Params:    inv.Params,
		PolicyDecision: evidence.PolicyDecision{
			Allow:     decision.Allow,
			RiskLevel: decision.RiskLevel,
			Reason:    decision.Reason,
			Advisory:  advisory,
		},
		ExecutionResult: evidence.ExecutionResult{
			Status:   result.Status,
			ExitCode: result.ExitCode,
		},
	}

	appendErr := s.evidence.Append(record)
	if appendErr != nil {
		return ExecuteOutput{
			OK:      false,
			EventID: record.EventID,
			Policy: PolicySummary{
				Allow:     decision.Allow,
				RiskLevel: decision.RiskLevel,
				Reason:    decision.Reason,
				PolicyRef: s.policyRef,
			},
			Execution: ExecutionSummary{
				Status:   "failed",
				ExitCode: nil,
				Stdout:   result.Stdout,
				Stderr:   appendErr.Error(),
			},
			Error: &ErrorSummary{
				Code:    "internal_error",
				Message: "failed to write evidence",
				Hint:    "Check evidence path permissions and disk state.",
			},
			Hints: []string{"evidence write failed; result treated as failed"},
		}
	}

	out := ExecuteOutput{
		OK:      ok,
		EventID: record.EventID,
		Policy: PolicySummary{
			Allow:     decision.Allow,
			RiskLevel: decision.RiskLevel,
			Reason:    decision.Reason,
			PolicyRef: s.policyRef,
		},
		Execution: ExecutionSummary{
			Status:   result.Status,
			ExitCode: result.ExitCode,
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
		},
		Error: errOut,
		Hints: hintsForExecution(result.Status, decision.RiskLevel),
	}
	return out
}

func decisionForDeny(reason string) policy.Decision {
	return policy.Decision{Allow: false, RiskLevel: "critical", Reason: reason}
}

func decisionForPolicyError() policy.Decision {
	return decisionForDeny("policy_evaluation_failed")
}

func boolPtr(v bool) *bool {
	return &v
}

func hintsForExecution(status, risk string) []string {
	hints := []string{}
	switch status {
	case "denied":
		hints = append(hints, "call get_event with event_id for full evidence details")
	case "failed":
		hints = append(hints, "inspect stderr and policy.reason for triage")
	case "success":
		hints = append(hints, "call get_event with event_id for immutable audit record")
	}
	if risk == "critical" {
		hints = append(hints, "critical risk operation; require explicit review in production workflows")
	}
	return hints
}
