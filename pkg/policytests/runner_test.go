package policytests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/policy"
)

type suiteFile struct {
	Pack   string     `json:"pack"`
	Policy string     `json:"policy"`
	Data   string     `json:"data,omitempty"`
	Cases  []testCase `json:"cases"`
}

type testCase struct {
	Name   string                    `json:"name"`
	Input  invocation.ToolInvocation `json:"input"`
	Expect expectedDecision          `json:"expect"`
}

type expectedDecision struct {
	Allow     bool   `json:"allow"`
	RiskLevel string `json:"risk_level"`
	Reason    string `json:"reason"`
}

func TestPackPoliciesFromJSONSuites(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	glob := filepath.Join(root, "packs", "_core", "ops", "*", "policy", "tests.json")
	paths, err := filepath.Glob(glob)
	if err != nil {
		t.Fatalf("glob tests.json: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("no policy test suites found under %q", glob)
	}
	sort.Strings(paths)

	for _, path := range paths {
		path := path
		t.Run(filepath.Base(filepath.Dir(filepath.Dir(path))), func(t *testing.T) {
			runSuite(t, path)
		})
	}
}

func runSuite(t *testing.T, suitePath string) {
	t.Helper()

	raw, err := os.ReadFile(suitePath)
	if err != nil {
		t.Fatalf("read suite %q: %v", suitePath, err)
	}

	var suite suiteFile
	if err := json.Unmarshal(raw, &suite); err != nil {
		t.Fatalf("parse suite %q: %v", suitePath, err)
	}
	if len(suite.Cases) == 0 {
		t.Fatalf("suite %q has no cases", suitePath)
	}

	policyFile := suite.Policy
	if policyFile == "" {
		policyFile = "policy.rego"
	}
	policyPath := filepath.Join(filepath.Dir(suitePath), policyFile)
	policyBytes, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("read policy %q: %v", policyPath, err)
	}

	var dataBytes []byte
	if suite.Data != "" {
		dataPath := filepath.Join(filepath.Dir(suitePath), suite.Data)
		dataBytes, err = os.ReadFile(dataPath)
		if err != nil {
			t.Fatalf("read data %q: %v", dataPath, err)
		}
	}

	engine, err := policy.NewOPAEngine(policyBytes, dataBytes)
	if err != nil {
		t.Fatalf("NewOPAEngine(%q): %v", suitePath, err)
	}

	for _, tc := range suite.Cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if err := tc.Input.ValidateStructure(); err != nil {
				t.Fatalf("invalid ToolInvocation structure: %v", err)
			}

			decision, err := engine.Evaluate(tc.Input)
			if err != nil {
				t.Fatalf("Evaluate() error: %v", err)
			}
			if decision.Allow != tc.Expect.Allow {
				t.Fatalf("allow mismatch: got %v want %v", decision.Allow, tc.Expect.Allow)
			}
			if decision.RiskLevel != tc.Expect.RiskLevel {
				t.Fatalf("risk_level mismatch: got %q want %q", decision.RiskLevel, tc.Expect.RiskLevel)
			}
			if decision.Reason != tc.Expect.Reason {
				t.Fatalf("reason mismatch: got %q want %q", decision.Reason, tc.Expect.Reason)
			}
		})
	}
}
