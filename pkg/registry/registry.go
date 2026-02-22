package registry

import (
	"fmt"
	"sort"
	"strings"
)

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

func (r *InMemoryRegistry) ToolNames() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}
