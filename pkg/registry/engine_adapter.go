package registry

import (
	"context"
	"fmt"

	"samebits.com/evidra-mcp/pkg/engine"
)

type resolverError struct {
	code string
	msg  string
}

func (e resolverError) Error() string { return e.msg }
func (e resolverError) Code() string  { return e.code }

type EngineToolResolver struct {
	reg Registry
}

func NewEngineToolResolver(reg Registry) *EngineToolResolver {
	return &EngineToolResolver{reg: reg}
}

func (r *EngineToolResolver) Resolve(tool string, op string) (engine.ToolDefinition, error) {
	def, ok := r.reg.Lookup(tool)
	if !ok {
		return nil, resolverError{
			code: "unregistered_tool",
			msg:  fmt.Sprintf("tool %q is not registered", tool),
		}
	}
	if !SupportsOperation(def, op) {
		return nil, resolverError{
			code: "unsupported_operation",
			msg:  fmt.Sprintf("operation %q is not supported for tool %q", op, tool),
		}
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
	raw := make(map[string]interface{}, len(params))
	for k, v := range params {
		raw[k] = v
	}
	return d.def.ValidateParams(d.op, raw)
}

func (d *engineToolDefinition) ValidateRawParams(params map[string]interface{}) error {
	return d.def.ValidateParams(d.op, params)
}

func (d *engineToolDefinition) BuildCommand(_ map[string]string) ([]string, error) {
	return nil, fmt.Errorf("build command is not supported for tool %q operation %q", d.def.Name, d.op)
}

func (d *engineToolDefinition) Metadata() engine.ToolMetadata {
	return engine.ToolMetadata{}
}

func (d *engineToolDefinition) Execute(ctx context.Context, params map[string]interface{}) (engine.ExecutionOutput, error) {
	res, err := d.def.Executor(ctx, ToolInvocationInput{
		Operation: d.op,
		Params:    params,
	})
	return engine.ExecutionOutput{
		Status:          res.Status,
		ExitCode:        res.ExitCode,
		Stdout:          res.Stdout,
		Stderr:          res.Stderr,
		StdoutTruncated: res.StdoutTruncated,
		StderrTruncated: res.StderrTruncated,
	}, err
}
