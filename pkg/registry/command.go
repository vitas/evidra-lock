package registry

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"samebits.com/evidra-mcp/pkg/tokens"
)

func NewDeclarativeCLIToolDefinition(name, inputSchema string, spec CLIToolSpec) (ToolDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return ToolDefinition{}, fmt.Errorf("tool name is required")
	}
	if strings.TrimSpace(spec.Binary) == "" {
		return ToolDefinition{}, fmt.Errorf("binary is required")
	}
	if len(spec.Operations) == 0 {
		return ToolDefinition{}, fmt.Errorf("at least one operation is required")
	}

	ops := make([]string, 0, len(spec.Operations))
	for op := range spec.Operations {
		ops = append(ops, op)
	}
	sort.Strings(ops)

	specCopy := spec
	return ToolDefinition{
		Name:                name,
		SupportedOperations: ops,
		InputSchema:         inputSchema,
		Metadata:            ToolMetadata{},
		ValidateParams: func(operation string, params map[string]interface{}) error {
			_, err := BuildDeclarativeCLIArgs(name, specCopy, operation, params)
			return err
		},
		Executor: func(ctx context.Context, inv ToolInvocationInput) (ExecutionResult, error) {
			argv, err := BuildDeclarativeCLIArgs(name, specCopy, inv.Operation, inv.Params)
			if err != nil {
				return ExecutionResult{Status: "failed", ExitCode: nil}, err
			}
			cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err = cmd.Run()
			if err == nil {
				code := 0
				return ExecutionResult{Status: "success", Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: &code}, nil
			}
			if exitErr, ok := err.(*exec.ExitError); ok {
				code := exitErr.ExitCode()
				return ExecutionResult{Status: "failed", Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: &code}, nil
			}
			return ExecutionResult{Status: "failed", Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: nil}, err
		},
	}, nil
}

func BuildDeclarativeCLIArgs(toolName string, spec CLIToolSpec, operation string, params map[string]interface{}) ([]string, error) {
	op, ok := spec.Operations[operation]
	if !ok {
		return nil, newInvalidParam(toolName, operation, "", "unsupported operation")
	}
	if params == nil {
		params = map[string]interface{}{}
	}

	out := []string{spec.Binary}
	allowed := map[string]bool{}
	for k := range op.Params {
		allowed[k] = true
	}
	for _, token := range op.Args {
		if err := tokens.ValidateTemplate(token, allowed); err != nil {
			return nil, err
		}
		placeholders, err := tokens.Placeholders(token)
		if err != nil {
			return nil, err
		}
		if len(placeholders) == 0 {
			out = append(out, token)
			continue
		}

		values := make(map[string]string, len(placeholders))
		skip := false
		for _, ph := range placeholders {
			rule, exists := op.Params[ph.Name]
			if !exists {
				return nil, newInvalidParam(toolName, operation, ph.Name, "undeclared placeholder")
			}
			paramVal, exists := params[ph.Name]
			if !exists {
				if ph.Optional || !rule.Required {
					skip = true
					break
				}
				return nil, newInvalidParam(toolName, operation, ph.Name, "missing required param")
			}
			arg, convErr := convertParam(toolName, operation, ph.Name, paramVal, rule.Type)
			if convErr != nil {
				return nil, convErr
			}
			if arg == "" && (ph.Optional || !rule.Required) {
				skip = true
				break
			}
			values[ph.Name] = arg
		}
		if skip {
			continue
		}

		expanded, err := tokens.ExpandTemplate(token, values)
		if err != nil {
			return nil, err
		}
		if expanded == "" {
			continue
		}
		out = append(out, expanded)
	}

	for name, rule := range op.Params {
		if !rule.Required {
			continue
		}
		if _, exists := params[name]; !exists {
			return nil, newInvalidParam(toolName, operation, name, "missing required param")
		}
	}
	return out, nil
}
