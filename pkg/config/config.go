package config

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

const (
	DefaultBundlePath = "policy/bundles/ops-v0.1"
	DefaultPolicyDir  = DefaultBundlePath
	DefaultPolicyPath = DefaultBundlePath + "/evidra/policy/policy.rego"
	DefaultDataPath   = DefaultBundlePath + "/evidra/data/params/data.json"
)

var (
	repoPolicyFallbackOnce sync.Once
)

// ResolveBundlePath resolves the OPA bundle directory path.
// Precedence: explicit flag > EVIDRA_BUNDLE_PATH env var.
// Returns "" if neither is set (caller should fall back to embedded bundle).
func ResolveBundlePath(flagValue string) string {
	if v := strings.TrimSpace(flagValue); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("EVIDRA_BUNDLE_PATH"))
}

func ResolvePolicyData(policyFlag, dataFlag string) (string, string, error) {
	policyFlag = strings.TrimSpace(policyFlag)
	dataFlag = strings.TrimSpace(dataFlag)
	if policyFlag != "" && dataFlag != "" {
		return policyFlag, dataFlag, nil
	}
	policyEnv := strings.TrimSpace(os.Getenv("EVIDRA_POLICY_PATH"))
	dataEnv := strings.TrimSpace(os.Getenv("EVIDRA_DATA_PATH"))
	if policyEnv != "" && dataEnv != "" {
		return policyEnv, dataEnv, nil
	}
	if fileExists(DefaultPolicyPath) && fileExists(DefaultDataPath) {
		repoPolicyFallbackOnce.Do(func() {
			fmt.Fprintln(os.Stderr, "warning: loading policy/data from repo fallback; set --policy/--data or env vars for production")
		})
		return DefaultPolicyPath, DefaultDataPath, nil
	}
	return "", "", fmt.Errorf("missing policy/data paths; provide --policy/--data or set EVIDRA_POLICY_PATH/EVIDRA_DATA_PATH")
}

// NormalizeEnvironment canonicalizes environment names to prevent
// silent policy mismatches (e.g. "prod" matching no by_env rules).
func NormalizeEnvironment(env string) string {
	v := strings.ToLower(strings.TrimSpace(env))
	switch v {
	case "prod", "prd":
		return "production"
	case "stg", "stage":
		return "staging"
	// NOTE: "dev" is NOT expanded to "development" — existing by_env overrides
	// commonly use "dev" as the canonical name. Expanding would silently disable
	// overrides unless data.json also lists "development".
	default:
		return v // unknown values (including "dev") pass through unchanged
	}
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}
