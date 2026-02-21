package ops

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"samebits.com/evidra-mcp/bundles/ops/evaluator"
	"samebits.com/evidra-mcp/bundles/ops/scenario"
	coreevidence "samebits.com/evidra-mcp/core/evidence"
	"samebits.com/evidra-mcp/core/runtime"
	"samebits.com/evidra-mcp/pkg/invocation"
)

// TODO(monorepo-split): publish bundles/ops as a standalone bundle repository.
// TODO(monorepo-split): move ops-specific policy profile and evidence adapters into the ops bundle repository.

const (
	DefaultPolicyPath   = "./bundles/ops/policies/policy.rego"
	DefaultEvidencePath = "./data/evidence"
)

type ValidationOutput struct {
	Pass       bool
	RiskLevel  string
	EvidenceID string
	Reasons    []string
	PolicyHits []string
}

func ValidateFile(path string) (ValidationOutput, error) {
	sc, err := scenario.LoadFile(path)
	if err != nil {
		return ValidationOutput{}, err
	}

	coreEval, err := runtime.NewEvaluator(DefaultPolicyPath, "")
	if err != nil {
		return ValidationOutput{}, err
	}
	opsEval := evaluator.New(coreEval)
	evalResult, err := opsEval.EvaluateScenario(sc)
	if err != nil {
		return ValidationOutput{}, err
	}

	store := coreevidence.NewStoreWithPath(evidencePath())
	if err := store.Init(); err != nil {
		return ValidationOutput{}, err
	}

	evidenceID := fmt.Sprintf("evt-%d", time.Now().UTC().UnixNano())
	rec := coreevidence.EvidenceRecord{
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
			"risk_level":     evalResult.RiskLevel,
			"action_count":   len(sc.Actions),
			"bundle_profile": "ops",
		},
		PolicyDecision: coreevidence.PolicyDecision{
			Allow:     evalResult.Pass,
			RiskLevel: evidenceRiskLevel(evalResult.RiskLevel),
			Reason:    primaryReason(evalResult.Reasons),
			Advisory:  false,
		},
		ExecutionResult: coreevidence.ExecutionResult{
			Status: passStatus(evalResult.Pass),
		},
	}

	if err := store.Append(rec); err != nil {
		return ValidationOutput{}, err
	}

	return ValidationOutput{
		Pass:       evalResult.Pass,
		RiskLevel:  evalResult.RiskLevel,
		EvidenceID: evidenceID,
		Reasons:    evalResult.Reasons,
		PolicyHits: evalResult.PolicyHits,
	}, nil
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

func evidenceRiskLevel(level string) string {
	if level == "high" {
		return "high"
	}
	return "low"
}

func evidencePath() string {
	if p := os.Getenv("EVIDRA_EVIDENCE_PATH"); p != "" {
		return p
	}
	return DefaultEvidencePath
}
