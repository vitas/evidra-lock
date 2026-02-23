package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	DefaultPolicyDir  = "policy/profiles/ops-v0.1"
	DefaultPolicyPath = DefaultPolicyDir + "/policy.rego"
	DefaultDataPath   = DefaultPolicyDir + "/data.json"
)

var (
	repoPolicyFallbackOnce sync.Once
	DefaultEvidenceDir     string
)

func init() {
	if home, err := os.UserHomeDir(); err == nil {
		DefaultEvidenceDir = filepath.Join(home, ".evidra", "evidence")
	} else {
		DefaultEvidenceDir = filepath.Join(".", "data", "evidence")
	}
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

func ResolveEvidenceDir(flagValue string) string {
	if path := strings.TrimSpace(flagValue); path != "" {
		return path
	}
	if dir := strings.TrimSpace(os.Getenv("EVIDRA_EVIDENCE_DIR")); dir != "" {
		return dir
	}
	if path := strings.TrimSpace(os.Getenv("EVIDRA_EVIDENCE_PATH")); path != "" {
		return path
	}
	return DefaultEvidenceDir
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}
