package plugins

import "samebits.com/evidra-mcp/pkg/registry"

// ToolPlugin registers one or more tool definitions into the registry.
type ToolPlugin interface {
	Name() string
	Register(r registry.Registry) error
}
