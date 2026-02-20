package kubectl

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"samebits.com/evidra-mcp/pkg/registry"
)

// Plugin is an experimental Level 2 compile-time plugin.
type Plugin struct{}

func New() Plugin {
	return Plugin{}
}

func (Plugin) Name() string {
	return "kubectl"
}

func (Plugin) Register(r registry.Registry) error {
	return r.RegisterTool(registry.ToolDefinition{
		Name:                "kubectl",
		SupportedOperations: []string{"get", "apply", "delete"},
		InputSchema:         `{"resource":"string","name":"string (delete only)","file":"string (apply only)","namespace":"string (optional, default 'default')"}`,
		Executor:            executeKubectl,
		ValidateParams:      validateKubectlParams,
	})
}

func validateKubectlParams(operation string, params map[string]interface{}) error {
	switch operation {
	case "get":
		if _, ok := stringParam(params, "resource", true); !ok {
			return fmt.Errorf("missing required param: resource")
		}
	case "apply":
		if _, ok := stringParam(params, "file", true); !ok {
			return fmt.Errorf("missing required param: file")
		}
	case "delete":
		if _, ok := stringParam(params, "resource", true); !ok {
			return fmt.Errorf("missing required param: resource")
		}
		if _, ok := stringParam(params, "name", true); !ok {
			return fmt.Errorf("missing required param: name")
		}
	default:
		return fmt.Errorf("unsupported operation")
	}

	if _, exists := params["namespace"]; exists {
		if _, ok := stringParam(params, "namespace", false); !ok {
			return fmt.Errorf("param namespace must be string")
		}
	}
	return nil
}

func executeKubectl(ctx context.Context, inv registry.ToolInvocationInput) (registry.ExecutionResult, error) {
	ns := "default"
	if v, ok := stringParam(inv.Params, "namespace", false); ok {
		if strings.TrimSpace(v) != "" {
			ns = v
		}
	}

	args := []string{}
	switch inv.Operation {
	case "get":
		resource, _ := stringParam(inv.Params, "resource", true)
		args = []string{"get", resource, "-n", ns}
	case "apply":
		file, _ := stringParam(inv.Params, "file", true)
		args = []string{"apply", "-f", file, "-n", ns}
	case "delete":
		resource, _ := stringParam(inv.Params, "resource", true)
		name, _ := stringParam(inv.Params, "name", true)
		args = []string{"delete", resource, name, "-n", ns}
	default:
		return registry.ExecutionResult{Status: "failed", ExitCode: nil}, fmt.Errorf("unsupported operation")
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		code := 0
		return registry.ExecutionResult{Status: "success", Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: &code}, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		code := exitErr.ExitCode()
		return registry.ExecutionResult{Status: "failed", Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: &code}, nil
	}
	return registry.ExecutionResult{Status: "failed", Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: nil}, err
}

func stringParam(params map[string]interface{}, key string, required bool) (string, bool) {
	v, ok := params[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	if required && strings.TrimSpace(s) == "" {
		return "", false
	}
	return s, true
}
