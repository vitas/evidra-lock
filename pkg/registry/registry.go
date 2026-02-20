package registry

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

type ExecutionResult struct {
	Status   string `json:"status"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode *int   `json:"exit_code"`
}

type Executor func(ctx context.Context, inv ToolInvocationInput) (ExecutionResult, error)

// ToolDefinition is an explicit static tool registration for v0.1.
type ToolDefinition struct {
	Name                string
	SupportedOperations []string
	// InputSchema is a minimal schema description for v0.1.
	InputSchema    string
	Executor       Executor
	ValidateParams func(operation string, params map[string]interface{}) error
}

type ToolInvocationInput struct {
	Operation string
	Params    map[string]interface{}
}

type Registry interface {
	Lookup(toolName string) (ToolDefinition, bool)
	RegisterTool(def ToolDefinition) error
}

type InMemoryRegistry struct {
	tools map[string]ToolDefinition
	order []string
}

func NewInMemoryRegistry(defs []ToolDefinition) *InMemoryRegistry {
	r := &InMemoryRegistry{
		tools: make(map[string]ToolDefinition, len(defs)),
		order: make([]string, 0, len(defs)),
	}
	for _, d := range defs {
		_ = r.RegisterTool(d)
	}
	return r
}

func NewDefaultRegistry() *InMemoryRegistry {
	r := NewInMemoryRegistry(nil)
	_ = r.RegisterTool(ToolDefinition{
		Name:                "echo",
		SupportedOperations: []string{"run"},
		InputSchema:         `{"text":"string"}`,
		Executor:            executeEcho,
		ValidateParams:      validateEchoParams,
	})
	_ = r.RegisterTool(ToolDefinition{
		Name:                "git",
		SupportedOperations: []string{"status"},
		InputSchema:         `{"path":"string (optional, default '.') "}`,
		Executor:            executeGitStatus,
		ValidateParams:      validateGitParams,
	})
	return r
}

func (r *InMemoryRegistry) Lookup(toolName string) (ToolDefinition, bool) {
	def, ok := r.tools[toolName]
	return def, ok
}

func (r *InMemoryRegistry) RegisterTool(def ToolDefinition) error {
	name := strings.TrimSpace(def.Name)
	if name == "" {
		return fmt.Errorf("tool name is required")
	}
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}
	if len(def.SupportedOperations) == 0 {
		return fmt.Errorf("tool %q must define supported operations", name)
	}
	if def.Executor == nil {
		return fmt.Errorf("tool %q must define executor", name)
	}
	if def.ValidateParams == nil {
		return fmt.Errorf("tool %q must define param validator", name)
	}
	def.Name = name
	r.tools[name] = def
	r.order = append(r.order, name)
	sort.Strings(r.order)
	return nil
}

func SupportsOperation(def ToolDefinition, operation string) bool {
	for _, op := range def.SupportedOperations {
		if op == operation {
			return true
		}
	}
	return false
}

func ValidateParams(def ToolDefinition, operation string, params map[string]interface{}) error {
	if def.ValidateParams == nil {
		return fmt.Errorf("tool %q has no param validator", def.Name)
	}
	return def.ValidateParams(operation, params)
}

func validateEchoParams(operation string, params map[string]interface{}) error {
	if operation != "run" {
		return fmt.Errorf("unsupported operation")
	}
	text, ok := params["text"]
	if !ok {
		return fmt.Errorf("missing required param: text")
	}
	if _, ok := text.(string); !ok {
		return fmt.Errorf("param text must be string")
	}
	return nil
}

func validateGitParams(operation string, params map[string]interface{}) error {
	if operation != "status" {
		return fmt.Errorf("unsupported operation")
	}
	if path, ok := params["path"]; ok {
		if _, ok := path.(string); !ok {
			return fmt.Errorf("param path must be string")
		}
	}
	return nil
}

func executeEcho(ctx context.Context, inv ToolInvocationInput) (ExecutionResult, error) {
	textValue, ok := inv.Params["text"]
	if !ok {
		return ExecutionResult{Status: "failed", ExitCode: nil}, fmt.Errorf("missing required param: text")
	}
	text, ok := textValue.(string)
	if !ok {
		return ExecutionResult{Status: "failed", ExitCode: nil}, fmt.Errorf("param text must be string")
	}

	cmd := exec.CommandContext(ctx, "echo", text)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		code := 0
		return ExecutionResult{
			Status:   "success",
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: &code,
		}, nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		code := exitErr.ExitCode()
		return ExecutionResult{
			Status:   "failed",
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: &code,
		}, nil
	}

	return ExecutionResult{
		Status:   "failed",
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: nil,
	}, err
}

func executeGitStatus(ctx context.Context, inv ToolInvocationInput) (ExecutionResult, error) {
	path := "."
	if raw, ok := inv.Params["path"]; ok {
		val, ok := raw.(string)
		if !ok {
			return ExecutionResult{Status: "failed", ExitCode: nil}, fmt.Errorf("param path must be string")
		}
		if strings.TrimSpace(val) != "" {
			path = val
		}
	}

	cmd := exec.CommandContext(ctx, "git", "-C", path, "status", "--porcelain")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		code := 0
		return ExecutionResult{
			Status:   "success",
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: &code,
		}, nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		code := exitErr.ExitCode()
		return ExecutionResult{
			Status:   "failed",
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: &code,
		}, nil
	}

	return ExecutionResult{
		Status:   "failed",
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: nil,
	}, err
}
