package registry

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// RegisterDevTools registers development/demo core tools.
func RegisterDevTools(r Registry) error {
	if err := r.RegisterTool(ToolDefinition{
		Name:                "echo",
		SupportedOperations: []string{"run"},
		InputSchema:         `{"text":"string"}`,
		Executor:            executeEcho,
		ValidateParams:      validateEchoParams,
	}); err != nil {
		return err
	}
	return r.RegisterTool(ToolDefinition{
		Name:                "git",
		SupportedOperations: []string{"status"},
		InputSchema:         `{"path":"string (optional, default '.') "}`,
		Executor:            executeGitStatus,
		ValidateParams:      validateGitParams,
	})
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
		if val, ok := raw.(string); ok && val != "" {
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
