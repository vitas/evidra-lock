package validate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
	opscfg "samebits.com/evidra-mcp/bundles/ops/config"
	"samebits.com/evidra-mcp/bundles/ops/evaluator"
	"samebits.com/evidra-mcp/bundles/ops/scenario"
	"samebits.com/evidra-mcp/bundles/ops/schema"
	"samebits.com/evidra-mcp/bundles/ops/validators"
	"samebits.com/evidra-mcp/pkg/config"
	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/runtime"
)

type Options struct {
	PolicyPath       string
	DataPath         string
	EvidenceDir      string
	ConfigPath       string
	EnableValidators *bool
	BuiltinFilter    map[string]bool
	ExecMode         string
	ExecFilter       map[string]bool
	BundleProfile    string
}

type Result struct {
	Pass        bool
	RiskLevel   string
	EvidenceID  string
	Reasons     []string
	PolicyHits  []string
	RuleIDs     []string
	Hints       []string
	ActionFacts []ActionFact
	Reports     []validators.Report
}

type ActionFact struct {
	Kind              string
	Namespace         string
	ResourceCount     int
	DestroyCount      int
	PublicExposure    bool
	ResourceAddresses []string
	ManifestKinds     []string
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
		return Result{}, err
	}
	sc := invocationToScenario(inv)
	return EvaluateScenario(ctx, sc, opts)
}

func EvaluateScenario(ctx context.Context, sc schema.Scenario, opts Options) (Result, error) {
	cfg, err := opscfg.Load(opts.ConfigPath)
	if err != nil {
		return Result{}, err
	}

	policyPath, dataPath, err := config.ResolvePolicyData(opts.PolicyPath, opts.DataPath)
	if err != nil {
		return Result{}, err
	}
	coreEval, err := runtime.NewEvaluator(policyPath, dataPath)
	if err != nil {
		return Result{}, err
	}
	opsEval := evaluator.New(coreEval)
	evalResult, err := opsEval.EvaluateScenario(sc)
	if err != nil {
		return Result{}, err
	}

	wd, _ := os.Getwd()
	validatorResult, validatorMeta, err := validators.RunForScenario(ctx, sc, wd, validators.RunOptions{
		Config:         cfg,
		EnableOverride: opts.EnableValidators,
		BuiltinFilter:  opts.BuiltinFilter,
		ExecMode:       opts.ExecMode,
		ExecFilter:     opts.ExecFilter,
	})
	if err != nil {
		return Result{}, err
	}

	finalPass := evalResult.Pass && validatorResult.Decision != "FAIL"
	finalRisk := evalResult.RiskLevel
	if validatorResult.RiskLevel == "high" || evalResult.RiskLevel == "high" {
		finalRisk = "high"
	}
	finalReasons := append([]string{}, evalResult.Reasons...)
	finalReasons = append(finalReasons, validatorResult.Reasons...)
	finalHints := append([]string{}, evalResult.Hints...)
	finalRuleIDs := append([]string{}, evalResult.RuleIDs...)
	finalHints = dedupeStrings(finalHints)
	finalRuleIDs = dedupeStrings(finalRuleIDs)

	evidenceDir := config.ResolveEvidenceDir(opts.EvidenceDir)
	store := evidence.NewStoreWithPath(evidenceDir)
	if err := store.Init(); err != nil {
		return Result{}, err
	}

	evidenceID := fmt.Sprintf("evt-%d", time.Now().UTC().UnixNano())
	bundleProfile := opts.BundleProfile
	if bundleProfile == "" {
		bundleProfile = "ops"
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
			"scenario_id":    sc.ScenarioID,
			"scenario_hash":  scenarioHash(sc),
			"policy_hits":    evalResult.PolicyHits,
			"rule_ids":       finalRuleIDs,
			"hints":          finalHints,
			"risk_level":     finalRisk,
			"decision":       passDecision(finalPass),
			"reasons":        finalReasons,
			"reports":        validatorResult.Reports,
			"validator_meta": validatorMeta,
			"action_count":   len(sc.Actions),
			"bundle_profile": bundleProfile,
		},
		PolicyDecision: evidence.PolicyDecision{
			Allow:     finalPass,
			RiskLevel: evidenceRiskLevel(finalRisk),
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

	if err := store.Append(rec); err != nil {
		return Result{}, err
	}

	return Result{
		Pass:        finalPass,
		RiskLevel:   finalRisk,
		EvidenceID:  evidenceID,
		Reasons:     dedupeStrings(finalReasons),
		PolicyHits:  evalResult.PolicyHits,
		RuleIDs:     finalRuleIDs,
		Hints:       finalHints,
		ActionFacts: collectActionFacts(sc.Actions),
		Reports:     validatorResult.Reports,
	}, nil
}

func invocationToScenario(inv invocation.ToolInvocation) schema.Scenario {
	return schema.Scenario{
		ScenarioID: scenarioIDFromInvocation(inv),
		Actor: schema.Actor{
			Type:   inv.Actor.Type,
			ID:     inv.Actor.ID,
			Origin: inv.Actor.Origin,
		},
		Source:    contextString(inv.Context, "source", inv.Actor.Origin),
		Timestamp: time.Now().UTC(),
		Actions: []schema.Action{
			{
				Kind:     fmt.Sprintf("%s.%s", inv.Tool, inv.Operation),
				Target:   mapFromValue(inv.Params["target"]),
				Intent:   contextString(inv.Context, "intent", ""),
				Payload:  copyMap(inv.Params),
				RiskTags: toStringSlice(inv.Params["risk_tags"]),
			},
		},
	}
}

func scenarioIDFromInvocation(inv invocation.ToolInvocation) string {
	if id := contextString(inv.Context, "scenario_id", ""); id != "" {
		return id
	}
	if id, ok := inv.Params["scenario_id"].(string); ok && strings.TrimSpace(id) != "" {
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
	return nil
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

func evidenceRiskLevel(level string) string {
	if level == "high" {
		return "high"
	}
	return "low"
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

func collectActionFacts(actions []schema.Action) []ActionFact {
	facts := make([]ActionFact, 0, len(actions))
	for _, action := range actions {
		fact := ActionFact{
			Kind:              action.Kind,
			Namespace:         namespaceForAction(action),
			ResourceCount:     intFromPayload(action.Payload, "resource_count"),
			DestroyCount:      intFromPayload(action.Payload, "destroy_count"),
			PublicExposure:    boolFromPayload(action.Payload, "publicly_exposed"),
			ResourceAddresses: stringSliceFromPayload(action.Payload, "resource_addresses"),
			ManifestKinds:     manifestKindsFromPayload(action.Payload),
		}
		facts = append(facts, fact)
	}
	return facts
}

func namespaceForAction(action schema.Action) string {
	if ns := stringFromMap(action.Payload, "namespace"); ns != "" {
		return ns
	}
	if ns := stringFromMap(action.Target, "namespace"); ns != "" {
		return ns
	}
	return ""
}

func stringFromMap(src map[string]interface{}, key string) string {
	if src == nil {
		return ""
	}
	if v, ok := src[key]; ok {
		if s, ok := v.(string); ok {
			return strings.ToLower(strings.TrimSpace(s))
		}
	}
	return ""
}

func intFromPayload(payload map[string]interface{}, key string) int {
	if payload == nil {
		return 0
	}
	switch v := payload[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case uint64:
		return int(v)
	}
	return 0
}

func boolFromPayload(payload map[string]interface{}, key string) bool {
	if payload == nil {
		return false
	}
	if v, ok := payload[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func stringSliceFromPayload(payload map[string]interface{}, key string) []string {
	if payload == nil {
		return nil
	}
	raw, ok := payload[key]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, entry := range v {
			if s, ok := entry.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func manifestKindsFromPayload(payload map[string]interface{}) []string {
	if payload == nil {
		return nil
	}
	inline, ok := payload["inline_yaml"].(string)
	if !ok || inline == "" {
		return nil
	}
	return parseYAMLKinds(inline)
}

func parseYAMLKinds(content string) []string {
	dec := yaml.NewDecoder(strings.NewReader(content))
	seen := map[string]struct{}{}
	for {
		var doc map[string]interface{}
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil
		}
		if len(doc) == 0 {
			continue
		}
		if kind, ok := doc["kind"].(string); ok && kind != "" {
			key := strings.ToLower(strings.TrimSpace(kind))
			seen[key] = struct{}{}
		}
	}
	kinds := make([]string, 0, len(seen))
	for k := range seen {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}
