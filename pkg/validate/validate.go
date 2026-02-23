package validate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"samebits.com/evidra-mcp/pkg/config"
	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/policysource"
	"samebits.com/evidra-mcp/pkg/runtime"
	"samebits.com/evidra-mcp/pkg/scenario"
)

// Sentinel errors returned by the Evaluate* functions.
// Use errors.Is() to distinguish error categories without string matching.
var (
	ErrInvalidInput  = errors.New("invalid_input")
	ErrPolicyFailure = errors.New("policy_failure")
	ErrEvidenceWrite = errors.New("evidence_write_failed")
)

type Options struct {
	PolicyPath   string
	DataPath     string
	EvidenceDir  string
	SkipEvidence bool
}

type Result struct {
	Pass       bool
	RiskLevel  string
	EvidenceID string
	Reasons    []string
	RuleIDs    []string
	Hints      []string
}

type scenarioEvaluation struct {
	Pass      bool
	RiskLevel string
	Reasons   []string
	RuleIDs   []string
	Hints     []string
	PolicyRef string
}

func EvaluateFile(ctx context.Context, path string, opts Options) (Result, error) {
	sc, err := scenario.LoadFile(path)
	if err != nil {
		return Result{}, err
	}
	return EvaluateScenario(ctx, sc, opts)
}

func EvaluateInvocation(ctx context.Context, inv invocation.ToolInvocation, opts Options) (Result, error) {
	if err := inv.ValidateStructure(); err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	sc := invocationToScenario(inv)
	return EvaluateScenario(ctx, sc, opts)
}

func EvaluateScenario(ctx context.Context, sc scenario.Scenario, opts Options) (Result, error) {
	policyPath, dataPath, err := config.ResolvePolicyData(opts.PolicyPath, opts.DataPath)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrPolicyFailure, err)
	}
	runtimeEval, err := runtime.NewEvaluator(policysource.NewLocalFileSource(policyPath, dataPath))
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrPolicyFailure, err)
	}
	evalResult, err := evaluateScenarioWithRuntime(ctx, runtimeEval, sc)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrPolicyFailure, err)
	}

	finalPass := evalResult.Pass
	finalRisk := evalResult.RiskLevel
	finalReasons := dedupeStrings(evalResult.Reasons)
	finalRuleIDs := dedupeStrings(evalResult.RuleIDs)
	finalHints := dedupeStrings(evalResult.Hints)

	var store *evidence.Store
	var evidenceID string
	if !opts.SkipEvidence {
		evidenceDir := config.ResolveEvidenceDir(opts.EvidenceDir)
		store = evidence.NewStoreWithPath(evidenceDir)
		if err := store.Init(); err != nil {
			return Result{}, fmt.Errorf("%w: %w", ErrEvidenceWrite, err)
		}
		evidenceID = fmt.Sprintf("evt-%d", time.Now().UTC().UnixNano())
	}
	rec := evidence.EvidenceRecord{
		EventID:   evidenceID,
		Timestamp: time.Now().UTC(),
		PolicyRef: evalResult.PolicyRef,
		Actor: invocation.Actor{
			Type:   sc.Actor.Type,
			ID:     actorID(sc.ScenarioID, sc.Actor.ID),
			Origin: sc.Source,
		},
		Tool:      "ops.scenario",
		Operation: "validate",
		Params: map[string]interface{}{
			"scenario_id":   sc.ScenarioID,
			"scenario_hash": scenarioHash(sc),
			"action_count":  len(sc.Actions),
		},
		PolicyDecision: evidence.PolicyDecision{
			Allow:     finalPass,
			RiskLevel: finalRisk,
			Reason:    primaryReason(finalReasons),
			Reasons:   dedupeStrings(finalReasons),
			Hints:     finalHints,
			RuleIDs:   finalRuleIDs,
			Advisory:  false,
		},
		ExecutionResult: evidence.ExecutionResult{
			Status: passStatus(finalPass),
		},
	}

	if store != nil {
		if err := store.Append(rec); err != nil {
			return Result{}, fmt.Errorf("%w: %w", ErrEvidenceWrite, err)
		}
	} else {
		evidenceID = ""
	}

	return Result{
		Pass:       finalPass,
		RiskLevel:  finalRisk,
		EvidenceID: evidenceID,
		Reasons:    dedupeStrings(finalReasons),
		RuleIDs:    finalRuleIDs,
		Hints:      finalHints,
	}, nil
}

func invocationToScenario(inv invocation.ToolInvocation) scenario.Scenario {
	return scenario.Scenario{
		ScenarioID: scenarioIDFromInvocation(inv),
		Actor: scenario.Actor{
			Type:   inv.Actor.Type,
			ID:     inv.Actor.ID,
			Origin: inv.Actor.Origin,
		},
		Source:    contextString(inv.Context, invocation.KeySource, inv.Actor.Origin),
		Timestamp: time.Now().UTC(),
		Actions: []scenario.Action{
			{
				Kind:     fmt.Sprintf("%s.%s", inv.Tool, inv.Operation),
				Target:   mapFromValue(inv.Params[invocation.KeyTarget]),
				Intent:   contextString(inv.Context, invocation.KeyIntent, ""),
				Payload:  mapFromValue(inv.Params[invocation.KeyPayload]),
				RiskTags: toStringSlice(inv.Params[invocation.KeyRiskTags]),
			},
		},
	}
}

func scenarioIDFromInvocation(inv invocation.ToolInvocation) string {
	if id := contextString(inv.Context, invocation.KeyScenarioID, ""); id != "" {
		return id
	}
	if id, ok := inv.Params[invocation.KeyScenarioID].(string); ok && strings.TrimSpace(id) != "" {
		return id
	}
	return fmt.Sprintf("%s.%s.%d", inv.Tool, inv.Operation, time.Now().UTC().UnixNano())
}

func contextString(ctx map[string]interface{}, key, fallback string) string {
	if ctx == nil {
		return fallback
	}
	if v, ok := ctx[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return fallback
}

func mapFromValue(value interface{}) map[string]interface{} {
	if m, ok := value.(map[string]interface{}); ok {
		return copyMap(m)
	}
	return map[string]interface{}{}
}

func copyMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	out := make(map[string]interface{}, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func toStringSlice(value interface{}) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, entry := range v {
			if s, ok := entry.(string); ok && s != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	}
	return nil
}

func scenarioHash(v interface{}) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func actorID(scenarioID, actorID string) string {
	if actorID != "" {
		return actorID
	}
	return scenarioID
}

func passStatus(pass bool) string {
	if pass {
		return "success"
	}
	return "denied"
}

func primaryReason(reasons []string) string {
	if len(reasons) == 0 {
		return "scenario_validated"
	}
	return reasons[0]
}

func passDecision(pass bool) string {
	if pass {
		return "PASS"
	}
	return "FAIL"
}

func dedupeStrings(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func evaluateScenarioWithRuntime(ctx context.Context, runtimeEval *runtime.Evaluator, sc scenario.Scenario) (scenarioEvaluation, error) {
	res := scenarioEvaluation{
		Pass:      true,
		RiskLevel: "low",
	}
	for i, action := range sc.Actions {
		tool, operation, ok := splitKind(action.Kind)
		if !ok {
			res.Pass = false
			res.RiskLevel = "high"
			reason := fmt.Sprintf("action[%d] invalid kind: %s", i, action.Kind)
			res.Reasons = append(res.Reasons, reason)
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
				invocation.KeyScenarioID: sc.ScenarioID,
				"action": map[string]interface{}{
					"kind":                action.Kind,
					invocation.KeyTarget:  action.Target,
					invocation.KeyIntent:  action.Intent,
					invocation.KeyPayload: action.Payload,
					invocation.KeyRiskTags: action.RiskTags,
				},
			},
			Context: map[string]interface{}{
				"timestamp":             sc.Timestamp.Format(time.RFC3339),
				invocation.KeySource:    sc.Source,
			},
		}

		decision, err := runtimeEval.EvaluateInvocation(inv)
		if err != nil {
			return scenarioEvaluation{}, err
		}
		if res.PolicyRef == "" {
			res.PolicyRef = decision.PolicyRef
		}
		if !decision.Allow {
			res.Pass = false
			res.RiskLevel = "high"
			if len(decision.Reasons) > 0 {
				res.Reasons = append(res.Reasons, decision.Reasons...)
			} else if decision.Reason != "" {
				res.Reasons = append(res.Reasons, decision.Reason)
			}
		}

		res.RuleIDs = append(res.RuleIDs, decision.Hits...)
		res.Hints = append(res.Hints, decision.Hints...)
		if !decision.Allow && len(decision.Hits) == 0 {
			res.RuleIDs = append(res.RuleIDs, "POL-UNLABELED-01")
		}
	}
	res.RuleIDs = dedupeStrings(res.RuleIDs)
	res.Hints = dedupeStrings(res.Hints)
	res.Reasons = dedupeStrings(res.Reasons)
	return res, nil
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
