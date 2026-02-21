package evaluator

import (
	"fmt"
	"strings"

	"samebits.com/evidra-mcp/bundles/ops/schema"
	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/runtime"
)

type Result struct {
	Pass       bool
	Reasons    []string
	PolicyHits []string
	RiskLevel  string
	PolicyRef  string
}

type Evaluator struct {
	scenarioEvaluator runtime.ScenarioEvaluator
}

func New(scenarioEvaluator runtime.ScenarioEvaluator) *Evaluator {
	return &Evaluator{scenarioEvaluator: scenarioEvaluator}
}

func (e *Evaluator) EvaluateScenario(sc schema.Scenario) (Result, error) {
	result := Result{Pass: true, Reasons: []string{}, PolicyHits: []string{}, RiskLevel: "normal"}
	for i, action := range sc.Actions {
		tool, operation, ok := splitKind(action.Kind)
		if !ok {
			result.Pass = false
			result.RiskLevel = "high"
			reason := fmt.Sprintf("action[%d] invalid kind: %s", i, action.Kind)
			result.Reasons = append(result.Reasons, reason)
			result.PolicyHits = append(result.PolicyHits, "invalid_action_kind")
			continue
		}

		actorID := strings.TrimSpace(sc.Actor.ID)
		if actorID == "" {
			actorID = sc.ScenarioID
		}

		inv := invocation.ToolInvocation{
			Actor: invocation.Actor{
				Type:   sc.Actor.Type,
				ID:     actorID,
				Origin: sc.Source,
			},
			Tool:      tool,
			Operation: operation,
			Params: map[string]interface{}{
				"scenario_id": sc.ScenarioID,
				"action": map[string]interface{}{
					"kind":      action.Kind,
					"target":    action.Target,
					"intent":    action.Intent,
					"payload":   action.Payload,
					"risk_tags": action.RiskTags,
				},
			},
			Context: map[string]interface{}{
				"timestamp": sc.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
				"source":    sc.Source,
			},
		}

		decision, err := e.scenarioEvaluator.EvaluateInvocation(inv)
		if err != nil {
			return Result{}, err
		}
		if result.PolicyRef == "" {
			result.PolicyRef = decision.PolicyRef
		}
		if !decision.Allow {
			result.Pass = false
			result.RiskLevel = "high"
			reason := fmt.Sprintf("action[%d] %s: %s", i, action.Kind, decision.Reason)
			result.Reasons = append(result.Reasons, reason)
			result.PolicyHits = append(result.PolicyHits, decision.Reason)
			continue
		}

		if decision.Reason == "autonomous-execution" {
			result.RiskLevel = "high"
			reason := fmt.Sprintf("action[%d] %s: autonomous execution path detected", i, action.Kind)
			result.Reasons = append(result.Reasons, reason)
			result.PolicyHits = append(result.PolicyHits, "autonomous-execution")
		}

		if hasTag(action.RiskTags, "breakglass") {
			result.RiskLevel = "high"
			result.PolicyHits = append(result.PolicyHits, "breakglass")
			result.Reasons = append(result.Reasons, fmt.Sprintf("action[%d] %s: breakglass tag present", i, action.Kind))
		}
	}
	if len(result.Reasons) == 0 {
		result.Reasons = append(result.Reasons, "all actions passed policy validation")
	}
	return result, nil
}

func splitKind(kind string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(kind), ".", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	if strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func hasTag(tags []string, target string) bool {
	for _, t := range tags {
		if t == target {
			return true
		}
	}
	return false
}
