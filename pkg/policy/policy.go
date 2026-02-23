package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

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
	LongRunning bool     `json:"long_running,omitempty"`
}

type Engine struct {
	query rego.PreparedEvalQuery
}

func NewOPAEngine(policyModules map[string][]byte, dataBytes []byte) (*Engine, error) {
	regoOpts := []func(*rego.Rego){
		rego.Query("data.evidra.policy.decision"),
	}
	if len(policyModules) == 0 {
		return nil, fmt.Errorf("policy source contains no modules")
	}
	filtered := map[string][]byte{}
	for name, module := range policyModules {
		if strings.HasPrefix(name, "tests/") || strings.Contains(name, "/tests/") {
			continue
		}
		filtered[name] = module
	}
	if len(filtered) > 0 && len(filtered) < len(policyModules) {
		policyModules = filtered
	}
	names := make([]string, 0, len(policyModules))
	for name := range policyModules {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		regoOpts = append(regoOpts, rego.Module(name, string(policyModules[name])))
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
	if actions := buildActionList(inv.Params); len(actions) > 0 {
		input["actions"] = actions
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
	case "low", "medium", "high", "critical", "normal":
		return true
	default:
		return false
	}
}

func buildActionList(params map[string]interface{}) []map[string]interface{} {
	if params == nil {
		return nil
	}
	var actions []map[string]interface{}
	appendRaw := func(raw interface{}) {
		switch v := raw.(type) {
		case []interface{}:
			for _, item := range v {
				if action, ok := normalizeAction(item); ok {
					actions = append(actions, action)
				}
			}
		case []map[string]interface{}:
			for _, action := range v {
				actions = append(actions, action)
			}
		default:
			if action, ok := normalizeAction(v); ok {
				actions = append(actions, action)
			}
		}
	}
	if raw, ok := params["actions"]; ok {
		appendRaw(raw)
	}
	if raw, ok := params["action"]; ok {
		appendRaw(raw)
	}
	return actions
}

func normalizeAction(raw interface{}) (map[string]interface{}, bool) {
	switch m := raw.(type) {
	case map[string]interface{}:
		return m, true
	default:
		return nil, false
	}
}
