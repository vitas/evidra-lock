package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage/inmem"

	"samebits.com/evidra-mcp/pkg/invocation"
)

type Decision struct {
	Allow       bool     `json:"allow"`
	RiskLevel   string   `json:"risk_level"`
	Reason      string   `json:"reason"`
	Reasons     []string `json:"reasons,omitempty"`
	Hints       []string `json:"hints,omitempty"`
	Hits        []string `json:"hits,omitempty"`
	Hint        string   `json:"hint,omitempty"`
	LongRunning bool     `json:"long_running,omitempty"`
}

type Engine struct {
	query rego.PreparedEvalQuery
}

func NewOPAEngine(policyBytes []byte, dataBytes []byte) (*Engine, error) {
	regoOpts := []func(*rego.Rego){
		rego.Query("data.evidra.policy.decision"),
		rego.Module("policy.rego", string(policyBytes)),
	}
	if len(dataBytes) > 0 {
		var dataObj map[string]interface{}
		if err := json.Unmarshal(dataBytes, &dataObj); err != nil {
			return nil, fmt.Errorf("parse policy data JSON: %w", err)
		}
		regoOpts = append(regoOpts, rego.Store(inmem.NewFromObject(dataObj)))
	}

	r := rego.New(regoOpts...)
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
	if reasons, ok := readStringSlice(out, "reasons"); ok {
		decision.Reasons = reasons
	}
	if hints, ok := readStringSlice(out, "hints"); ok {
		decision.Hints = hints
	}
	if hits, ok := readStringSlice(out, "hits"); ok {
		decision.Hits = hits
	}
	if hint, ok := out["hint"].(string); ok {
		decision.Hint = hint
	}
	if decision.Hint == "" && len(decision.Hints) > 0 {
		decision.Hint = decision.Hints[0]
	}
	if len(decision.Reasons) == 0 && decision.Reason != "" {
		decision.Reasons = []string{decision.Reason}
	}
	if longRunning, ok := out["long_running"].(bool); ok {
		decision.LongRunning = longRunning
	}
	return decision, nil
}

func readStringSlice(m map[string]interface{}, key string) ([]string, bool) {
	raw, exists := m[key]
	if !exists {
		return nil, false
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s, ok := item.(string)
		if !ok || s == "" {
			continue
		}
		out = append(out, s)
	}
	return out, true
}

func isValidRiskLevel(level string) bool {
	switch level {
	case "low", "medium", "high", "critical":
		return true
	default:
		return false
	}
}
