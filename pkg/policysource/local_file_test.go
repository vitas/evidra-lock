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

	src := NewLocalFileSource(path, "")
	ref1, err := src.PolicyRef()
	if err != nil {
		t.Fatalf("PolicyRef() error = %v", err)
	}
	ref2, err := src.PolicyRef()
	if err != nil {
		t.Fatalf("PolicyRef() second call error = %v", err)
	}

	if ref1 == "" {
		t.Fatalf("expected non-empty policy ref")
	}
	if ref1 != ref2 {
		t.Fatalf("expected stable policy ref, got %q != %q", ref1, ref2)
	}
}
