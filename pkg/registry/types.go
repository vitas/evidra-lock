package registry

import "context"

// ExecutionResult describes the output of a tool executor.
type ExecutionResult struct {
	Status          string `json:"status"`
	Stdout          string `json:"stdout,omitempty"`
	Stderr          string `json:"stderr,omitempty"`
	StdoutTruncated bool   `json:"stdout_truncated,omitempty"`
	StderrTruncated bool   `json:"stderr_truncated,omitempty"`
	ExitCode        *int   `json:"exit_code"`
}

// Executor runs a registered tool operation.
type Executor func(ctx context.Context, inv ToolInvocationInput) (ExecutionResult, error)

// ToolInvocationInput holds the operation and params passed to an executor.
type ToolInvocationInput struct {
	Operation string
	Params    map[string]interface{}
}

// ToolMetadata carries descriptive labels about a tool operation.
type ToolMetadata struct {
	LongRunning bool
	Destructive bool
	Labels      []string
}

// ToolDefinition describes how to validate and execute an operation.
type ToolDefinition struct {
	Name                string
	SupportedOperations []string
	InputSchema         string
	Metadata            ToolMetadata
	Executor            Executor
	ValidateParams      func(operation string, params map[string]interface{}) error
}

// ParamRule defines the expected type and requirement for a parameter.
type ParamRule struct {
	Type     string
	Required bool
}

// CLIOperationSpec describes CLI args and param rules for an operation.
type CLIOperationSpec struct {
	Args   []string
	Params map[string]ParamRule
}

// CLIToolSpec defines a CLI tool with binary and operations.
type CLIToolSpec struct {
	Binary     string
	Operations map[string]CLIOperationSpec
}

// Registry tracks registered tools.
type Registry interface {
	Lookup(toolName string) (ToolDefinition, bool)
	RegisterTool(def ToolDefinition) error
	ToolNames() []string
}
