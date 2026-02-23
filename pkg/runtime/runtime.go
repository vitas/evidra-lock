package runtime

import (
	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/policy"
	"samebits.com/evidra-mcp/pkg/policysource"
)

type ScenarioEvaluator interface {
	EvaluateInvocation(inv invocation.ToolInvocation) (policy.Decision, error)
}

type Evaluator struct {
	engine    *policy.Engine
	policyRef string
}

func NewEvaluator(policyPath, dataPath string) (*Evaluator, error) {
	src := policysource.NewLocalFileSource(policyPath, dataPath)
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
	return &Evaluator{engine: eng, policyRef: ref}, nil
}

func (e *Evaluator) EvaluateInvocation(inv invocation.ToolInvocation) (policy.Decision, error) {
	if err := inv.ValidateStructure(); err != nil {
		return policy.Decision{}, err
	}
	d, err := e.engine.Evaluate(inv)
	if err != nil {
		return policy.Decision{}, err
	}
	d.PolicyRef = e.policyRef
	return d, nil
}
