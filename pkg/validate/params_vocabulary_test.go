package validate_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParamsVocabulary verifies that all entries in ops.destructive_operations
// are well-formed "tool.operation" pairs. Each entry must have exactly one dot
// with non-empty tool and operation parts. This prevents malformed entries like
// "k8s" (no dot), ".apply" (empty tool), or "kubectl." (empty operation).
func TestParamsVocabulary(t *testing.T) {
	t.Parallel()

	paramsFile := filepath.Join("..", "..", "policy", "bundles", "ops-v0.1",
		"evidra", "data", "params", "data.json")

	data, err := os.ReadFile(paramsFile)
	if err != nil {
		t.Fatalf("cannot read params: %v", err)
	}

	var params map[string]any
	if err := json.Unmarshal(data, &params); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	opsParam, ok := params["ops.destructive_operations"]
	if !ok {
		t.Fatal("ops.destructive_operations not found in params")
	}

	byEnv := opsParam.(map[string]any)["by_env"].(map[string]any)
	for env, val := range byEnv {
		ops := val.([]any)
		for _, op := range ops {
			kind := op.(string)
			parts := strings.SplitN(kind, ".", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				t.Errorf("ops.destructive_operations[%s]: malformed kind %q — expected tool.operation", env, kind)
			}
		}
	}
}
