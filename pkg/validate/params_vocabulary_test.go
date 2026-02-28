package validate_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParamsVocabulary verifies that all entries in ops.destructive_operations
// use allowed kind prefixes. Catches "k8s.apply" or "tf.destroy" in params.
func TestParamsVocabulary(t *testing.T) {
	t.Parallel()

	allowedPrefixes := []string{
		"kubectl.", "terraform.", "helm.", "argocd.",
	}

	paramsFile := filepath.Join("..", "..", "policy", "bundles", "ops-v0.1",
		"evidra", "data", "params", "data.json")

	data, err := os.ReadFile(paramsFile)
	if err != nil {
		t.Fatalf("cannot read params: %v", err)
	}

	var params map[string]interface{}
	if err := json.Unmarshal(data, &params); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	opsParam, ok := params["ops.destructive_operations"]
	if !ok {
		t.Fatal("ops.destructive_operations not found in params")
	}

	byEnv := opsParam.(map[string]interface{})["by_env"].(map[string]interface{})
	for env, val := range byEnv {
		ops := val.([]interface{})
		for _, op := range ops {
			kind := op.(string)
			allowed := false
			for _, prefix := range allowedPrefixes {
				if strings.HasPrefix(kind, prefix) {
					allowed = true
					break
				}
			}
			if !allowed {
				t.Errorf("ops.destructive_operations[%s]: unauthorized kind %q", env, kind)
			}
		}
	}
}
