package validate_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestKindVocabulary scans all rego rules for action.kind matches and
// verifies they only use allowed prefixes. This prevents "kind drift"
// where domain rules use k8s.apply instead of kubectl.apply, creating
// silent policy bypasses.
//
// Scope: this test checks only kind LITERALS in action.kind == "..."
// patterns. It does not cover kinds referenced in helper data or
// derived dynamically. If new tool prefixes are added (e.g. crossplane),
// update allowedPrefixes — this friction is intentional.
func TestKindVocabulary(t *testing.T) {
	t.Parallel()

	allowedPrefixes := []string{
		"kubectl.", "terraform.", "helm.", "argocd.",
	}

	// Patterns that match kind literals in rego
	kindPattern := regexp.MustCompile(`action\.kind\s*==\s*"([^"]+)"`)

	rulesDir := filepath.Join("..", "..", "policy", "bundles", "ops-v0.1",
		"evidra", "policy", "rules")

	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		t.Fatalf("cannot read rules dir: %v", err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".rego") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(rulesDir, entry.Name()))
		if err != nil {
			t.Fatalf("cannot read %s: %v", entry.Name(), err)
		}

		matches := kindPattern.FindAllSubmatch(data, -1)
		for _, m := range matches {
			kind := string(m[1])
			allowed := false
			for _, prefix := range allowedPrefixes {
				if strings.HasPrefix(kind, prefix) {
					allowed = true
					break
				}
			}
			if !allowed {
				t.Errorf("%s: unauthorized kind %q — use one of %v prefixes",
					entry.Name(), kind, allowedPrefixes)
			}
		}
	}
}
