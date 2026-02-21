package regulated

import (
	coreif "samebits.com/evidra-mcp/core/interfaces"
	"samebits.com/evidra-mcp/core/runtime"
)

// TODO(monorepo-split): publish bundles/regulated as a standalone bundle repository.
// TODO(monorepo-split): switch DefaultPolicyPath to a regulated profile when available.

const (
	DefaultPolicyPath = "./policy/profiles/ops-v0.1/policy.rego"
	DefaultDataPath   = "./policy/profiles/ops-v0.1/data.json"
)

func ValidateFile(path string) (coreif.ScenarioDecision, error) {
	eval, err := runtime.NewEvaluator(DefaultPolicyPath, DefaultDataPath)
	if err != nil {
		return coreif.ScenarioDecision{}, err
	}
	inv, err := runtime.ReadInvocationFile(path)
	if err != nil {
		return coreif.ScenarioDecision{}, err
	}
	return eval.EvaluateInvocation(inv)
}
