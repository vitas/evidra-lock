package validate_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"samebits.com/evidra/pkg/validate"
)

// TestKindVocabulary scans all rego rules for action.kind matches and
// verifies they only use prefixes derived from ops.destructive_operations
// in params/data.json. This prevents "kind drift" where domain rules
// use unauthorized prefixes, creating silent policy bypasses.
//
// To add a new tool prefix: add its destructive operations to
// ops.destructive_operations in data.json — this test picks it up
// automatically.
func TestKindVocabulary(t *testing.T) {
	t.Parallel()

	paramsFile := filepath.Join("..", "..", "policy", "bundles", "ops-v0.1",
		"evidra", "data", "params", "data.json")

	allowedPrefixes, err := validate.AllowedPrefixesFromParams(paramsFile)
	if err != nil {
		t.Fatalf("load prefixes: %v", err)
	}
	if len(allowedPrefixes) == 0 {
		t.Fatal("no prefixes found — ops.destructive_operations empty?")
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
