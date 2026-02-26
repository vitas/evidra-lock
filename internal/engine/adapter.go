package engine

import (
	"context"
	"fmt"

	"samebits.com/evidra/pkg/invocation"
	"samebits.com/evidra/pkg/policy"
	"samebits.com/evidra/pkg/runtime"
)

// Adapter wraps a runtime.Evaluator for use by the API server.
// It validates invocation structure before evaluation.
type Adapter struct {
	evaluator *runtime.Evaluator
}

// NewAdapter creates an Adapter by initializing a runtime.Evaluator from the given PolicySource.
// The evaluator is created once and reused for all requests.
func NewAdapter(src runtime.PolicySource) (*Adapter, error) {
	eval, err := runtime.NewEvaluator(src)
	if err != nil {
		return nil, fmt.Errorf("engine.NewAdapter: %w", err)
	}
	return &Adapter{evaluator: eval}, nil
}

// Evaluate validates the invocation structure and evaluates it against the policy bundle.
// The context parameter is reserved for future use (tracing, cancellation).
func (a *Adapter) Evaluate(_ context.Context, inv invocation.ToolInvocation) (policy.Decision, error) {
	if err := inv.ValidateStructure(); err != nil {
		return policy.Decision{}, fmt.Errorf("engine.Evaluate: %w", err)
	}
	dec, err := a.evaluator.EvaluateInvocation(inv)
	if err != nil {
		return policy.Decision{}, fmt.Errorf("engine.Evaluate: %w", err)
	}
	return dec, nil
}

// BundleRevision returns the bundle revision from the underlying evaluator.
func (a *Adapter) BundleRevision() string {
	return a.evaluator.BundleRevision()
}

// ProfileName returns the profile name from the underlying evaluator.
func (a *Adapter) ProfileName() string {
	return a.evaluator.ProfileName()
}
