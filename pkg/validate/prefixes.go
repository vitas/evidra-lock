package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// AllowedPrefixesFromParams reads ops.destructive_operations from
// the params data.json and returns the unique tool prefixes.
// Example: ["kubectl.delete","helm.upgrade"] -> ["helm.","kubectl."]
func AllowedPrefixesFromParams(paramsPath string) ([]string, error) {
	data, err := os.ReadFile(paramsPath)
	if err != nil {
		return nil, fmt.Errorf("validate.AllowedPrefixesFromParams: %w", err)
	}

	var params map[string]any
	if err := json.Unmarshal(data, &params); err != nil {
		return nil, fmt.Errorf("validate.AllowedPrefixesFromParams: %w", err)
	}

	opsParam, ok := params["ops.destructive_operations"]
	if !ok {
		return nil, fmt.Errorf("validate.AllowedPrefixesFromParams: ops.destructive_operations not found")
	}

	opsMap, ok := opsParam.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("validate.AllowedPrefixesFromParams: ops.destructive_operations is not an object")
	}

	byEnv, ok := opsMap["by_env"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("validate.AllowedPrefixesFromParams: ops.destructive_operations.by_env is not an object")
	}

	seen := map[string]bool{}
	for _, envOps := range byEnv {
		ops, ok := envOps.([]any)
		if !ok {
			continue
		}
		for _, op := range ops {
			kind, ok := op.(string)
			if !ok {
				continue
			}
			parts := strings.SplitN(kind, ".", 2)
			if len(parts) == 2 {
				seen[parts[0]+"."] = true
			}
		}
	}

	prefixes := make([]string, 0, len(seen))
	for p := range seen {
		prefixes = append(prefixes, p)
	}
	return prefixes, nil
}
