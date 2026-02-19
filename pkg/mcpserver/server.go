package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/executor"
)

var ErrPolicyDenied = errors.New("command denied by policy")

type PolicyEngine interface {
	IsAllowed(command string) bool
}

type CommandExecutor interface {
	Execute(ctx context.Context, command string, args []string) (executor.Result, error)
}

type EvidenceStore interface {
	Append(record evidence.EvidenceRecord) (evidence.EvidenceRecord, error)
}

type Options struct {
	Name    string
	Version string
}

type ExecuteInput struct {
	Command string   `json:"command" jsonschema:"Command to execute"`
	Args    []string `json:"args" jsonschema:"Arguments for command"`
}

type ExecuteOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

type executeHandler struct {
	service *ExecuteService
}

func NewServer(opts Options, policyEngine PolicyEngine, cmdExecutor CommandExecutor, evidenceStore EvidenceStore) *mcp.Server {
	if opts.Name == "" {
		opts.Name = "evidra-mcp"
	}
	if opts.Version == "" {
		opts.Version = "v0.1.0"
	}

	svc := NewExecuteService(policyEngine, cmdExecutor, evidenceStore)
	handler := &executeHandler{service: svc}

	server := mcp.NewServer(
		&mcp.Implementation{Name: opts.Name, Version: opts.Version},
		nil,
	)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "execute",
		Description: "Execute an allowed command with policy and evidence checks",
	}, handler.Handle)
	return server
}

func (h *executeHandler) Handle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input ExecuteInput,
) (*mcp.CallToolResult, ExecuteOutput, error) {
	output, err := h.service.Execute(ctx, input)
	if err != nil {
		if errors.Is(err, ErrPolicyDenied) {
			return nil, ExecuteOutput{}, fmt.Errorf("execute denied: %w", err)
		}
		return nil, ExecuteOutput{}, err
	}
	return nil, output, nil
}

type ExecuteService struct {
	policy   PolicyEngine
	executor CommandExecutor
	evidence EvidenceStore
}

func NewExecuteService(policyEngine PolicyEngine, cmdExecutor CommandExecutor, evidenceStore EvidenceStore) *ExecuteService {
	return &ExecuteService{
		policy:   policyEngine,
		executor: cmdExecutor,
		evidence: evidenceStore,
	}
}

func (s *ExecuteService) Execute(ctx context.Context, input ExecuteInput) (ExecuteOutput, error) {
	if !s.policy.IsAllowed(input.Command) {
		if err := s.appendEvidence(input, executor.Result{ExitCode: -1}, "denied"); err != nil {
			return ExecuteOutput{}, err
		}
		return ExecuteOutput{}, ErrPolicyDenied
	}

	result, err := s.executor.Execute(ctx, input.Command, input.Args)
	if err != nil {
		if appendErr := s.appendEvidence(input, executor.Result{ExitCode: -1}, "execution_error"); appendErr != nil {
			return ExecuteOutput{}, appendErr
		}
		return ExecuteOutput{}, err
	}

	status := "completed"
	if result.TimedOut {
		status = "timeout"
	} else if result.ExitCode != 0 {
		status = "failed"
	}
	if err := s.appendEvidence(input, result, status); err != nil {
		return ExecuteOutput{}, err
	}

	return ExecuteOutput{
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		ExitCode: result.ExitCode,
	}, nil
}

func (s *ExecuteService) appendEvidence(input ExecuteInput, result executor.Result, status string) error {
	details := map[string]interface{}{
		"status":    status,
		"command":   input.Command,
		"args":      input.Args,
		"stdout":    result.Stdout,
		"stderr":    result.Stderr,
		"exit_code": result.ExitCode,
		"timed_out": result.TimedOut,
	}

	_, err := s.evidence.Append(evidence.EvidenceRecord{
		ID:      fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Actor:   "mcp.execute",
		Action:  "execute",
		Subject: input.Command,
		Details: details,
	})
	return err
}
