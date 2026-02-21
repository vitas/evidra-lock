package regulated

import (
	"samebits.com/evidra-mcp/pkg/runtime"
)

// TODO(monorepo-split): publish bundles/regulated as a standalone bundle repository.
// TODO(monorepo-split): switch DefaultPolicyPath to a regulated profile when available.

const (
	DefaultPolicyPath = "./policy/profiles/ops-v0.1/policy.rego"
	DefaultDataPath   = "./policy/profiles/ops-v0.1/data.json"
)

func ValidateFile(path string) (runtime.ScenarioDecision, error) {
	eval, err := runtime.NewEvaluator(DefaultPolicyPath, DefaultDataPath)
	if err != nil {
		return runtime.ScenarioDecision{}, err
	}
	inv, err := runtime.ReadInvocationFile(path)
	if err != nil {
		return runtime.ScenarioDecision{}, err
	}
	return eval.EvaluateInvocation(inv)
}
