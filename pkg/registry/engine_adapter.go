package registry

import (
	"fmt"

	"samebits.com/evidra-mcp/pkg/engine"
)

type EngineToolResolver struct {
	reg Registry
}

func NewEngineToolResolver(reg Registry) *EngineToolResolver {
	return &EngineToolResolver{reg: reg}
}

func (r *EngineToolResolver) Resolve(tool string, op string) (engine.ToolDefinition, error) {
	def, err := ResolveOperation(r.reg, tool, op)
	if err != nil {
		return nil, err
	}
	return &engineToolDefinition{def: def, op: op}, nil
}

type engineToolDefinition struct {
	def ToolDefinition
	op  string
}

func (d *engineToolDefinition) Name() string {
	return d.def.Name
}

func (d *engineToolDefinition) Operation() string {
	return d.op
}

func (d *engineToolDefinition) ValidateParams(params map[string]string) error {
	return d.def.ValidateParams(d.op, stringParamsToInterface(params))
}

func (d *engineToolDefinition) BuildCommand(params map[string]string) ([]string, error) {
	if d.def.BuildCommand == nil {
		return nil, fmt.Errorf("build command is not supported for tool %q operation %q", d.def.Name, d.op)
	}
	return d.def.BuildCommand(d.op, params)
}

func (d *engineToolDefinition) Metadata() engine.ToolMetadata {
	return engine.ToolMetadata{
		LongRunning: d.def.Metadata.LongRunning,
		Destructive: d.def.Metadata.Destructive,
		Labels:      d.def.Metadata.Labels,
	}
}
