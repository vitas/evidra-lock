package regulated

import (
	"os"

	"samebits.com/evidra-mcp/pkg/runtime"
)


const (
	regulatedPolicyPath = "./policy/profiles/regulated-v0.1/policy.rego"
	opsPolicyPath       = "./policy/profiles/ops-v0.1/policy.rego"
	DefaultDataPath     = "./policy/profiles/ops-v0.1/data.json"
)

var DefaultPolicyPath = defaultPolicyPath()

func defaultPolicyPath() string {
	if _, err := os.Stat(regulatedPolicyPath); err == nil {
		return regulatedPolicyPath
	}
	return opsPolicyPath
}

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
