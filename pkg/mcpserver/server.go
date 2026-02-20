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
	Status   string `json:"status"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode *int   `json:"exit_code"`
	EventID  string `json:"event_id"`
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
		Description: "Invoke a registered tool through registry, policy, and evidence flow",
	}, executeTool.Handle)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_event",
		Description: "Fetch a single immutable evidence record by event_id",
	}, getEventTool.Handle)
	return server
}

func (h *executeHandler) Handle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input invocation.ToolInvocation,
) (*mcp.CallToolResult, ExecuteOutput, error) {
	output, err := h.service.Execute(ctx, input)
	if err != nil {
		return nil, output, err
	}
	return nil, output, nil
}

func (h *getEventHandler) Handle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input getEventInput,
) (*mcp.CallToolResult, evidence.Record, error) {
	record, err := h.service.GetEvent(ctx, input.EventID)
	if err != nil {
		return nil, evidence.Record{}, err
	}
	return nil, record, nil
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

func (s *ExecuteService) GetEvent(_ context.Context, eventID string) (evidence.Record, error) {
	if eventID == "" {
		return evidence.Record{}, errors.New("event_id is required")
	}

	rec, found, err := evidence.FindByEventID(s.evidencePath, eventID)
	if err != nil {
		if errors.Is(err, evidence.ErrChainInvalid) {
			return evidence.Record{}, errors.New("evidence_chain_invalid")
		}
		return evidence.Record{}, err
	}
	if !found {
		return evidence.Record{}, fmt.Errorf("event_id %q not found", eventID)
	}
	return rec, nil
}

func (s *ExecuteService) Execute(ctx context.Context, inv invocation.ToolInvocation) (ExecuteOutput, error) {
	if err := inv.ValidateStructure(); err != nil {
		return s.denyWithEvidence(inv, "invalid_invocation", err)
	}

	def, ok := s.registry.Lookup(inv.Tool)
	if !ok {
		return s.denyWithEvidence(inv, "unregistered_tool", fmt.Errorf("tool %q is not registered", inv.Tool))
	}
	if !registry.SupportsOperation(def, inv.Operation) {
		return s.denyWithEvidence(inv, "unsupported_operation", fmt.Errorf("operation %q is not supported for tool %q", inv.Operation, inv.Tool))
	}
	if err := registry.ValidateParams(inv.Tool, inv.Operation, inv.Params); err != nil {
		return s.denyWithEvidence(inv, "invalid_params", err)
	}

	decision, evalErr := s.policy.Evaluate(inv)
	if evalErr != nil {
		decision = decisionForPolicyError()
	}
	if s.mode == ModeEnforce && !decision.Allow {
		return s.writeFinal(inv, decision, registry.ExecutionResult{Status: "denied", ExitCode: nil}, errors.New(decision.Reason), false)
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
		return s.writeFinal(inv, decision, execResult, execErr, advisory)
	}

	return s.writeFinal(inv, decision, execResult, nil, advisory)
}

func (s *ExecuteService) denyWithEvidence(inv invocation.ToolInvocation, reason string, err error) (ExecuteOutput, error) {
	return s.writeFinal(
		inv,
		decisionForDeny(reason),
		registry.ExecutionResult{Status: "denied", ExitCode: nil},
		err,
		false,
	)
}

func (s *ExecuteService) writeFinal(
	inv invocation.ToolInvocation,
	decision policy.Decision,
	result registry.ExecutionResult,
	callErr error,
	advisory bool,
) (ExecuteOutput, error) {
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
			Status:   "failed",
			Stdout:   result.Stdout,
			Stderr:   appendErr.Error(),
			ExitCode: nil,
			EventID:  record.EventID,
		}, fmt.Errorf("write evidence: %w", appendErr)
	}

	out := ExecuteOutput{
		Status:   result.Status,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		ExitCode: result.ExitCode,
		EventID:  record.EventID,
	}
	return out, callErr
}

func decisionForDeny(reason string) policy.Decision {
	return policy.Decision{Allow: false, RiskLevel: "critical", Reason: reason}
}

func decisionForPolicyError() policy.Decision {
	return decisionForDeny("policy_evaluation_failed")
}
