package policy

import (
	"context"
	"errors"
	"fmt"

	"github.com/open-policy-agent/opa/rego"

	"samebits.com/evidra-mcp/pkg/invocation"
)

type Decision struct {
	Allow     bool   `json:"allow"`
	RiskLevel string `json:"risk_level"`
	Reason    string `json:"reason"`
}

type Engine struct {
	query rego.PreparedEvalQuery
}

func LoadFromFile(path string) (*Engine, error) {
	return LoadFromFiles(path, nil)
}

func LoadFromFiles(policyPath string, dataPaths []string) (*Engine, error) {
	paths := make([]string, 0, 1+len(dataPaths))
	paths = append(paths, policyPath)
	paths = append(paths, dataPaths...)

	r := rego.New(
		rego.Query("data.evidra.policy.decision"),
		rego.Load(paths, nil),
	)

	query, err := r.PrepareForEval(context.Background())
	if err != nil {
		return nil, fmt.Errorf("prepare policy query: %w", err)
	}

	return &Engine{query: query}, nil
}

func (e *Engine) Evaluate(inv invocation.ToolInvocation) (Decision, error) {
	input := map[string]interface{}{
		"actor": map[string]interface{}{
			"type":   inv.Actor.Type,
			"id":     inv.Actor.ID,
			"origin": inv.Actor.Origin,
		},
		"tool":      inv.Tool,
		"operation": inv.Operation,
		"params":    inv.Params,
		"context":   inv.Context,
	}

	results, err := e.query.Eval(context.Background(), rego.EvalInput(input))
	if err != nil {
		return Decision{Allow: false, RiskLevel: "critical", Reason: "policy_evaluation_failed"}, err
	}
	if len(results) == 0 || len(results[0].Expressions) == 0 {
		return Decision{Allow: false, RiskLevel: "critical", Reason: "policy_evaluation_failed"}, errors.New("policy decision not found")
	}

	out, ok := results[0].Expressions[0].Value.(map[string]interface{})
	if !ok {
		return Decision{Allow: false, RiskLevel: "critical", Reason: "policy_evaluation_failed"}, errors.New("policy decision has invalid type")
	}

	decision := Decision{
		Allow:     false,
		RiskLevel: "critical",
		Reason:    "policy_evaluation_failed",
	}

	allow, ok := out["allow"].(bool)
	if !ok {
		return decision, errors.New("policy decision allow is missing or invalid")
	}
	reason, ok := out["reason"].(string)
	if !ok || reason == "" {
		return decision, errors.New("policy decision reason is missing or invalid")
	}
	riskLevel, ok := out["risk_level"].(string)
	if !ok || !isValidRiskLevel(riskLevel) {
		return decision, errors.New("policy decision risk_level is missing or invalid")
	}

	decision.Allow = allow
	decision.RiskLevel = riskLevel
	decision.Reason = reason
	return decision, nil
}

func isValidRiskLevel(level string) bool {
	switch level {
	case "low", "medium", "high", "critical":
		return true
	default:
		return false
	}
}
