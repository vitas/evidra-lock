package runtime

import (
	"encoding/json"
	"fmt"
	"os"

	coreif "samebits.com/evidra-mcp/core/interfaces"
	corepolicy "samebits.com/evidra-mcp/core/policy"
	corepolicysource "samebits.com/evidra-mcp/core/policysource"
	"samebits.com/evidra-mcp/pkg/invocation"
)

// TODO(monorepo-split): move core/runtime evaluator and contracts into standalone core module.

type Evaluator struct {
	engine    *corepolicy.Engine
	policyRef string
}

func NewEvaluator(policyPath, dataPath string) (*Evaluator, error) {
	src := corepolicysource.NewLocalFileSource(policyPath, dataPath)
	policyBytes, err := src.LoadPolicy()
	if err != nil {
		return nil, err
	}
	dataBytes, err := src.LoadData()
	if err != nil {
		return nil, err
	}
	eng, err := corepolicy.NewOPAEngine(policyBytes, dataBytes)
	if err != nil {
		return nil, err
	}
	ref, err := src.PolicyRef()
	if err != nil {
		return nil, err
	}
	return &Evaluator{engine: eng, policyRef: ref}, nil
}

func (e *Evaluator) EvaluateInvocation(inv invocation.ToolInvocation) (coreif.ScenarioDecision, error) {
	if err := inv.ValidateStructure(); err != nil {
		return coreif.ScenarioDecision{}, err
	}
	d, err := e.engine.Evaluate(inv)
	if err != nil {
		return coreif.ScenarioDecision{}, err
	}
	return coreif.ScenarioDecision{Allow: d.Allow, RiskLevel: d.RiskLevel, Reason: d.Reason, PolicyRef: e.policyRef}, nil
}

func ReadInvocationFile(path string) (invocation.ToolInvocation, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return invocation.ToolInvocation{}, fmt.Errorf("read invocation file: %w", err)
	}
	var inv invocation.ToolInvocation
	if err := json.Unmarshal(raw, &inv); err != nil {
		return invocation.ToolInvocation{}, fmt.Errorf("parse invocation JSON: %w", err)
	}
	return inv, nil
}
