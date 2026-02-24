package runtime

import (
	"samebits.com/evidra/pkg/invocation"
	"samebits.com/evidra/pkg/policy"
)

// PolicySource is the interface for loading policy modules, data, and a stable
// content-addressed reference. Callers inject a concrete implementation into
// NewEvaluator; pkg/policysource.LocalFileSource satisfies this interface.
type PolicySource interface {
	LoadPolicy() (map[string][]byte, error)
	LoadData() ([]byte, error)
	PolicyRef() (string, error)
	BundleRevision() string
	ProfileName() string
}

type ScenarioEvaluator interface {
	EvaluateInvocation(inv invocation.ToolInvocation) (policy.Decision, error)
}

type Evaluator struct {
	engine         *policy.Engine
	policyRef      string
	bundleRevision string
	profileName    string
}

func NewEvaluator(src PolicySource) (*Evaluator, error) {
	policyModules, err := src.LoadPolicy()
	if err != nil {
		return nil, err
	}
	dataBytes, err := src.LoadData()
	if err != nil {
		return nil, err
	}
	eng, err := policy.NewOPAEngine(policyModules, dataBytes)
	if err != nil {
		return nil, err
	}
	ref, err := src.PolicyRef()
	if err != nil {
		return nil, err
	}
	return &Evaluator{
		engine:         eng,
		policyRef:      ref,
		bundleRevision: src.BundleRevision(),
		profileName:    src.ProfileName(),
	}, nil
}

func (e *Evaluator) EvaluateInvocation(inv invocation.ToolInvocation) (policy.Decision, error) {
	d, err := e.engine.Evaluate(inv)
	if err != nil {
		return policy.Decision{}, err
	}
	d.PolicyRef = e.policyRef
	d.BundleRevision = e.bundleRevision
	d.ProfileName = e.profileName
	return d, nil
}

// BundleRevision returns the bundle revision from the policy source.
func (e *Evaluator) BundleRevision() string { return e.bundleRevision }

// ProfileName returns the profile name from the policy source.
func (e *Evaluator) ProfileName() string { return e.profileName }
