package validators

import (
	"context"

	"samebits.com/evidra-mcp/bundles/ops/schema"
)

// TODO(monorepo-split): move validator contracts to a shared module once the interface is stable.
type Validator interface {
	Name() string
	Applicable(actionKind string) bool
	Run(ctx context.Context, action schema.Action, workdir string) (Report, error)
}

type Registry struct {
	validators []Validator
}

func (r *Registry) Register(v Validator) {
	if v == nil {
		return
	}
	r.validators = append(r.validators, v)
}

func (r *Registry) ForAction(kind string) []Validator {
	out := make([]Validator, 0, len(r.validators))
	for _, v := range r.validators {
		if v.Applicable(kind) {
			out = append(out, v)
		}
	}
	return out
}
