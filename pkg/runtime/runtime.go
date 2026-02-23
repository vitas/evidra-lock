package runtime

import (
	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/policy"
	"samebits.com/evidra-mcp/pkg/policysource"
)

type ScenarioDecision struct {
	Allow     bool     `json:"allow"`
	RiskLevel string   `json:"risk_level"`
	Reason    string   `json:"reason"`
	PolicyRef string   `json:"policy_ref,omitempty"`
	Hits      []string `json:"hits,omitempty"`
	Hints     []string `json:"hints,omitempty"`
	Reasons   []string `json:"reasons,omitempty"`
}

type ScenarioEvaluator interface {
	EvaluateInvocation(inv invocation.ToolInvocation) (ScenarioDecision, error)
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

func (e *Evaluator) EvaluateInvocation(inv invocation.ToolInvocation) (ScenarioDecision, error) {
	if err := inv.ValidateStructure(); err != nil {
		return ScenarioDecision{}, err
	}
	d, err := e.engine.Evaluate(inv)
	if err != nil {
		return ScenarioDecision{}, err
	}
	return ScenarioDecision{
		Allow:     d.Allow,
		RiskLevel: d.RiskLevel,
		Reason:    d.Reason,
		PolicyRef: e.policyRef,
		Hits:      d.Hits,
		Hints:     d.Hints,
		Reasons:   d.Reasons,
	}, nil
}
