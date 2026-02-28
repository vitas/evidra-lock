package validate_test

import (
	"context"
	"testing"

	"samebits.com/evidra/pkg/scenario"
	"samebits.com/evidra/pkg/validate"
)

// contractRules lists rule IDs that are part of the public contract.
// Renaming any of these is a BREAKING CHANGE — agents and CI scripts
// may parse these IDs.
var contractRules = map[string]scenario.Scenario{
	"ops.insufficient_context": killswitchScenario("terraform.apply",
		nil, map[string]interface{}{}, nil),
	"ops.unknown_destructive": killswitchScenario("pulumi.up",
		nil, map[string]interface{}{}, nil),
	"k8s.protected_namespace": killswitchScenario("kubectl.delete",
		map[string]interface{}{"namespace": "kube-system"},
		map[string]interface{}{"namespace": "kube-system", "resource": "pod"}, nil),
}

func TestRuleIDContract(t *testing.T) {
	t.Parallel()
	for wantID, sc := range contractRules {
		t.Run(wantID, func(t *testing.T) {
			t.Parallel()
			opts := safeOpts(t)
			opts.SkipEvidence = true
			result, err := validate.EvaluateScenario(context.Background(), sc, opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !containsString(result.RuleIDs, wantID) {
				t.Errorf("rule ID %q not found in result.RuleIDs=%v", wantID, result.RuleIDs)
			}
		})
	}
}
