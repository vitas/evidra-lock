package registry

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"samebits.com/evidra-mcp/pkg/tokens"
)

type ExecutionResult struct {
	Status          string `json:"status"`
	Stdout          string `json:"stdout,omitempty"`
	Stderr          string `json:"stderr,omitempty"`
	StdoutTruncated bool   `json:"stdout_truncated,omitempty"`
	StderrTruncated bool   `json:"stderr_truncated,omitempty"`
	ExitCode        *int   `json:"exit_code"`
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
	ToolNames() []string
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
	_ = RegisterDevTools(r)
	return r
}

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

func (r *InMemoryRegistry) ToolNames() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
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

type ParamRule struct {
	Type     string
	Required bool
}

type CLIOperationSpec struct {
	Args   []string
	Params map[string]ParamRule
}

type CLIToolSpec struct {
	Binary     string
	Operations map[string]CLIOperationSpec
}

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
		ValidateParams: func(operation string, params map[string]interface{}) error {
			_, err := BuildDeclarativeCLIArgs(specCopy, operation, params)
			return err
		},
		Executor: func(ctx context.Context, inv ToolInvocationInput) (ExecutionResult, error) {
			argv, err := BuildDeclarativeCLIArgs(specCopy, inv.Operation, inv.Params)
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

func BuildDeclarativeCLIArgs(spec CLIToolSpec, operation string, params map[string]interface{}) ([]string, error) {
	op, ok := spec.Operations[operation]
	if !ok {
		return nil, fmt.Errorf("unsupported operation")
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
		placeholders := tokens.Placeholders(token)
		if len(placeholders) == 0 {
			out = append(out, token)
			continue
		}

		values := make(map[string]string, len(placeholders))
		skip := false
		for _, ph := range placeholders {
			rule, exists := op.Params[ph.Name]
			if !exists {
				return nil, fmt.Errorf("placeholder %q is not declared in params", ph.Name)
			}
			paramVal, exists := params[ph.Name]
			if !exists {
				if ph.Optional || !rule.Required {
					skip = true
					break
				}
				return nil, fmt.Errorf("missing required param: %s", ph.Name)
			}
			arg, convErr := convertParamToArg(ph.Name, paramVal, rule.Type)
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
			return nil, fmt.Errorf("missing required param: %s", name)
		}
	}
	return out, nil
}

func convertParamToArg(name string, value interface{}, typ string) (string, error) {
	switch typ {
	case "string":
		v, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("param %s must be string", name)
		}
		if strings.Contains(v, "\n") || strings.ContainsRune(v, rune(0)) {
			return "", fmt.Errorf("param %s contains disallowed characters", name)
		}
		if name == "url" && strings.TrimSpace(v) != "" {
			if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
				return "", fmt.Errorf("param %s must start with http:// or https://", name)
			}
		}
		return v, nil
	case "int":
		switch x := value.(type) {
		case int:
			return strconv.Itoa(x), nil
		case int64:
			return strconv.FormatInt(x, 10), nil
		case float64:
			if math.Trunc(x) != x {
				return "", fmt.Errorf("param %s must be int", name)
			}
			return strconv.FormatInt(int64(x), 10), nil
		default:
			return "", fmt.Errorf("param %s must be int", name)
		}
	case "bool":
		v, ok := value.(bool)
		if !ok {
			return "", fmt.Errorf("param %s must be bool", name)
		}
		return strconv.FormatBool(v), nil
	default:
		return "", fmt.Errorf("unsupported param type %q", typ)
	}
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
