package registry

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
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
	InputSchema string
	Executor    Executor
}

type ToolInvocationInput struct {
	Operation string
	Params    map[string]interface{}
}

type Registry interface {
	Lookup(toolName string) (ToolDefinition, bool)
}

type InMemoryRegistry struct {
	tools map[string]ToolDefinition
}

func NewInMemoryRegistry(defs []ToolDefinition) *InMemoryRegistry {
	tools := make(map[string]ToolDefinition, len(defs))
	for _, d := range defs {
		tools[d.Name] = d
	}
	return &InMemoryRegistry{tools: tools}
}

func NewDefaultRegistry() *InMemoryRegistry {
	return NewInMemoryRegistry([]ToolDefinition{
		{
			Name:                "echo",
			SupportedOperations: []string{"run"},
			InputSchema:         `{"text":"string"}`,
			Executor:            executeEcho,
		},
		{
			Name:                "git",
			SupportedOperations: []string{"status"},
			InputSchema:         `{"path":"string (optional, default '.') "}`,
			Executor:            executeGitStatus,
		},
	})
}

func (r *InMemoryRegistry) Lookup(toolName string) (ToolDefinition, bool) {
	def, ok := r.tools[toolName]
	return def, ok
}

func SupportsOperation(def ToolDefinition, operation string) bool {
	for _, op := range def.SupportedOperations {
		if op == operation {
			return true
		}
	}
	return false
}

func ValidateParams(toolName, operation string, params map[string]interface{}) error {
	switch toolName {
	case "echo":
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
	case "git":
		if operation != "status" {
			return fmt.Errorf("unsupported operation")
		}
		if path, ok := params["path"]; ok {
			if _, ok := path.(string); !ok {
				return fmt.Errorf("param path must be string")
			}
		}
	default:
		return fmt.Errorf("unsupported tool")
	}
	return nil
}

func executeEcho(_ context.Context, inv ToolInvocationInput) (ExecutionResult, error) {
	textValue, ok := inv.Params["text"]
	if !ok {
		return ExecutionResult{Status: "failed", ExitCode: nil}, fmt.Errorf("missing required param: text")
	}
	text, ok := textValue.(string)
	if !ok {
		return ExecutionResult{Status: "failed", ExitCode: nil}, fmt.Errorf("param text must be string")
	}

	code := 0
	return ExecutionResult{
		Status:   "success",
		Stdout:   text + "\n",
		Stderr:   "",
		ExitCode: &code,
	}, nil
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

	cmd := exec.CommandContext(ctx, "git", "-C", path, "status")
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
