package validate_test

import (
	"context"
	"testing"

	"samebits.com/evidra/pkg/validate"
)

// TestBundleIntegrity verifies that the ops-v0.1 bundle loads and
// produces a decision. This catches broken bundle layout, missing
// files, or invalid rego after refactors.
func TestBundleIntegrity(t *testing.T) {
	t.Parallel()
	sc := safeScenario()
	opts := safeOpts(t)
	opts.SkipEvidence = true

	result, err := validate.EvaluateScenario(context.Background(), sc, opts)
	if err != nil {
		t.Fatalf("bundle failed to load or evaluate: %v", err)
	}
	// safeScenario is a known-good input — must produce a decision
	if result.RiskLevel == "" {
		t.Error("RiskLevel empty — bundle may not have evaluated")
	}
}
