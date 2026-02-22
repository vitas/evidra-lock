package registry

// ToolMetadata carries descriptive labels about a tool operation.
type ToolMetadata struct {
	LongRunning bool
	Destructive bool
	Labels      []string
}

// ToolDefinition describes how to validate and build commands for an operation.
type ToolDefinition struct {
	Name                string
	SupportedOperations []string
	InputSchema         string
	Metadata            ToolMetadata
	BuildCommand        func(operation string, params map[string]string) ([]string, error)
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
