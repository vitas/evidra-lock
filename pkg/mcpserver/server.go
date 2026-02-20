package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/policy"
	"samebits.com/evidra-mcp/pkg/registry"
)

type EvidenceStore interface {
	Append(record evidence.EvidenceRecord) (evidence.EvidenceRecord, error)
}

type Options struct {
	Name    string
	Version string
}

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

func NewServer(opts Options, reg registry.Registry, policyEngine *policy.Engine, evidenceStore EvidenceStore) *mcp.Server {
	if opts.Name == "" {
		opts.Name = "evidra-mcp"
	}
	if opts.Version == "" {
		opts.Version = "v0.1.0"
	}

	svc := NewExecuteService(reg, policyEngine, evidenceStore)
	handler := &executeHandler{service: svc}

	server := mcp.NewServer(
		&mcp.Implementation{Name: opts.Name, Version: opts.Version},
		nil,
	)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "execute",
		Description: "Invoke a registered tool through registry, policy, and evidence flow",
	}, handler.Handle)
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

type ExecuteService struct {
	registry registry.Registry
	policy   *policy.Engine
	evidence EvidenceStore
}

func NewExecuteService(reg registry.Registry, policyEngine *policy.Engine, evidenceStore EvidenceStore) *ExecuteService {
	return &ExecuteService{
		registry: reg,
		policy:   policyEngine,
		evidence: evidenceStore,
	}
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
		decision = policy.Decision{Allow: false, RiskLevel: "critical", Reason: "policy_evaluation_failed"}
	}
	if !decision.Allow {
		return s.writeFinal(inv, decision, registry.ExecutionResult{Status: "denied", ExitCode: nil}, errors.New(decision.Reason))
	}

	execResult, execErr := def.Executor(ctx, registry.ToolInvocationInput{
		Operation: inv.Operation,
		Params:    inv.Params,
	})
	if execErr != nil {
		execResult.Status = "failed"
		if execResult.Stderr == "" {
			execResult.Stderr = execErr.Error()
		}
		return s.writeFinal(inv, decision, execResult, execErr)
	}

	return s.writeFinal(inv, decision, execResult, nil)
}

func (s *ExecuteService) denyWithEvidence(inv invocation.ToolInvocation, reason string, err error) (ExecuteOutput, error) {
	return s.writeFinal(
		inv,
		policy.Decision{Allow: false, RiskLevel: "critical", Reason: reason},
		registry.ExecutionResult{Status: "denied", ExitCode: nil},
		err,
	)
}

func (s *ExecuteService) writeFinal(
	inv invocation.ToolInvocation,
	decision policy.Decision,
	result registry.ExecutionResult,
	callErr error,
) (ExecuteOutput, error) {
	record := evidence.EvidenceRecord{
		EventID:   fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Timestamp: time.Now().UTC(),
		Actor:     inv.Actor,
		Tool:      inv.Tool,
		Operation: inv.Operation,
		Params:    inv.Params,
		PolicyDecision: evidence.PolicyDecision{
			Allow:     decision.Allow,
			RiskLevel: decision.RiskLevel,
			Reason:    decision.Reason,
		},
		ExecutionResult: evidence.ExecutionResult{
			Status:   result.Status,
			ExitCode: result.ExitCode,
		},
	}

	appended, appendErr := s.evidence.Append(record)
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
		EventID:  appended.EventID,
	}
	return out, callErr
}
