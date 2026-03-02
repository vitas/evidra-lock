package policy

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestPolicyBoundary_NoInputActionsOutsideCanonicalizer(t *testing.T) {
	t.Parallel()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to determine current test file path")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
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
			if strings.Contains(line, "input.actions") {
				rel, relErr := filepath.Rel(repoRoot, path)
				if relErr != nil {
					rel = path
				}
				offenders = append(offenders, rel+":"+strconv.Itoa(lineNum))
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking policy files: %v", err)
	}

	if len(offenders) > 0 {
		t.Fatalf("policy boundary violation: input.actions found outside canonicalizer: %v", offenders)
	}
}
