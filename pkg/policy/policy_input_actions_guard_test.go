package policy

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestPolicyBoundary_NoInputActionsOutsideCanonicalizer(t *testing.T) {
	t.Parallel()

	repoRoot, err := repoRootFromCaller()
	if err != nil {
		t.Fatal(err)
	}
	offenders, err := collectPolicyOffenders(repoRoot, []string{"input.actions"})
	if err != nil {
		t.Fatalf("walking policy files: %v", err)
	}

	if len(offenders) > 0 {
		t.Fatalf("policy boundary violation: input.actions found outside canonicalizer: %v", offenders)
	}
}

func TestPolicyBoundary_NoK8sPathOrCasingOutsideCanonicalizer(t *testing.T) {
	t.Parallel()

	repoRoot, err := repoRootFromCaller()
	if err != nil {
		t.Fatal(err)
	}
	patterns := []string{
		"spec.template.spec",
		"spec.jobTemplate.spec.template.spec",
		"metadata.namespace",
		"securityContext",
		"hostPID",
		"hostNetwork",
		"hostIPC",
	}
	offenders, err := collectPolicyOffenders(repoRoot, patterns)
	if err != nil {
		t.Fatalf("walking policy files: %v", err)
	}

	if len(offenders) > 0 {
		t.Fatalf("policy boundary violation: k8s shape/casing found outside canonicalizer: %v", offenders)
	}
}

func repoRootFromCaller() (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("failed to determine current test file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..")), nil
}

func collectPolicyOffenders(repoRoot string, patterns []string) ([]string, error) {
	policyDir := filepath.Join(repoRoot, "policy", "bundles", "ops-v0.1", "evidra", "policy")
	var offenders []string

	err := filepath.WalkDir(policyDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".rego" {
			return nil
		}
		if filepath.Base(path) == "canonicalize.rego" {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			for _, pattern := range patterns {
				if strings.Contains(line, pattern) {
					rel, relErr := filepath.Rel(repoRoot, path)
					if relErr != nil {
						rel = path
					}
					offenders = append(offenders, rel+":"+strconv.Itoa(lineNum)+":"+pattern)
					break
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
		return nil
	})
	return offenders, err
}
