package registry

import pkgregistry "samebits.com/evidra-mcp/pkg/registry"

// TODO(monorepo-split): move core/registry and tool contracts into standalone core module.

type ExecutionResult = pkgregistry.ExecutionResult
type Executor = pkgregistry.Executor
type ToolDefinition = pkgregistry.ToolDefinition
type ToolInvocationInput = pkgregistry.ToolInvocationInput
type Registry = pkgregistry.Registry
type InMemoryRegistry = pkgregistry.InMemoryRegistry

type ParamRule = pkgregistry.ParamRule
type CLIOperationSpec = pkgregistry.CLIOperationSpec
type CLIToolSpec = pkgregistry.CLIToolSpec

var (
	NewInMemoryRegistry             = pkgregistry.NewInMemoryRegistry
	NewDefaultRegistry              = pkgregistry.NewDefaultRegistry
	RegisterDevTools                = pkgregistry.RegisterDevTools
	SupportsOperation               = pkgregistry.SupportsOperation
	ValidateParams                  = pkgregistry.ValidateParams
	NewDeclarativeCLIToolDefinition = pkgregistry.NewDeclarativeCLIToolDefinition
	BuildDeclarativeCLIArgs         = pkgregistry.BuildDeclarativeCLIArgs
)
