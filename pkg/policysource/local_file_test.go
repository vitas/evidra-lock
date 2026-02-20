package policysource

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPolicyRefStableForSameContent(t *testing.T) {
	temp := t.TempDir()
	path := filepath.Join(temp, "policy.rego")
	content := []byte(`package evidra.policy
import rego.v1
decision := {"allow": false, "risk_level": "critical", "reason": "policy_denied_default"}
`)

	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile(policy) error = %v", err)
	}

	src := NewLocalFilePolicySource(path)
	ref1 := src.PolicyRef()
	ref2 := src.PolicyRef()

	if ref1 == "" {
		t.Fatalf("expected non-empty policy ref")
	}
	if ref1 != ref2 {
		t.Fatalf("expected stable policy ref, got %q != %q", ref1, ref2)
	}
}
