package corpus_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"samebits.com/evidra/pkg/bundlesource"
	"samebits.com/evidra/pkg/invocation"
	"samebits.com/evidra/pkg/runtime"
)

var bundleDir = filepath.Join("..", "..", "policy", "bundles", "ops-v0.1")

// corpusCase represents a single test case from a corpus JSON file.
type corpusCase struct {
	Meta struct {
		ID       string   `json:"id"`
		Rules    []string `json:"rules"`
		Priority string   `json:"priority"`
	} `json:"_meta"`
	Input *struct {
		Actor       invocation.Actor       `json:"actor"`
		Tool        string                 `json:"tool"`
		Operation   string                 `json:"operation"`
		Environment string                 `json:"environment"`
		Params      map[string]interface{} `json:"params"`
		Context     map[string]interface{} `json:"context"`
	} `json:"input"`
	Expect *struct {
		Allow         *bool    `json:"allow"`
		RiskLevel     string   `json:"risk_level,omitempty"`
		RuleIDsContain []string `json:"rule_ids_contain,omitempty"`
		RuleIDsAbsent  []string `json:"rule_ids_absent,omitempty"`
		HintsMinCount  int      `json:"hints_min_count"`
	} `json:"expect"`
}

func TestCorpus(t *testing.T) {
	t.Parallel()

	src, err := bundlesource.NewBundleSource(bundleDir)
	if err != nil {
		t.Fatalf("NewBundleSource: %v", err)
	}
	evaluator, err := runtime.NewEvaluator(src)
	if err != nil {
		t.Fatalf("NewEvaluator: %v", err)
	}

	cases := loadCorpusCases(t)
	if len(cases) == 0 {
		t.Fatal("no corpus cases found")
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Meta.ID, func(t *testing.T) {
			t.Parallel()

			if tc.Input == nil || tc.Expect == nil {
				t.Skip("no input/expect — agent-only case")
			}

			inv := invocation.ToolInvocation{
				Actor:       tc.Input.Actor,
				Tool:        tc.Input.Tool,
				Operation:   tc.Input.Operation,
				Params:      tc.Input.Params,
				Context:     tc.Input.Context,
				Environment: tc.Input.Environment,
			}

			decision, err := evaluator.EvaluateInvocation(inv)
			if err != nil {
				t.Fatalf("EvaluateInvocation: %v", err)
			}

			// Assert allow/deny
			if tc.Expect.Allow != nil {
				if decision.Allow != *tc.Expect.Allow {
					t.Errorf("allow: got %v, want %v (reason: %s)", decision.Allow, *tc.Expect.Allow, decision.Reason)
				}
			}

			// Assert risk_level
			if tc.Expect.RiskLevel != "" {
				if decision.RiskLevel != tc.Expect.RiskLevel {
					t.Errorf("risk_level: got %q, want %q", decision.RiskLevel, tc.Expect.RiskLevel)
				}
			}

			// Assert rule_ids_contain (hits must include these)
			for _, wantRule := range tc.Expect.RuleIDsContain {
				if !containsString(decision.Hits, wantRule) {
					t.Errorf("hits missing %q, got %v", wantRule, decision.Hits)
				}
			}

			// Assert rule_ids_absent (hits must NOT include these)
			for _, absentRule := range tc.Expect.RuleIDsAbsent {
				if containsString(decision.Hits, absentRule) {
					t.Errorf("hits should not contain %q, got %v", absentRule, decision.Hits)
				}
			}

			// Assert hints_min_count
			if len(decision.Hints) < tc.Expect.HintsMinCount {
				t.Errorf("hints count: got %d, want >= %d, hints: %v", len(decision.Hints), tc.Expect.HintsMinCount, decision.Hints)
			}
		})
	}
}

func loadCorpusCases(t *testing.T) []corpusCase {
	t.Helper()

	corpusDir := "."
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		t.Fatalf("read corpus dir: %v", err)
	}

	var cases []corpusCase
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		// Skip non-case files
		switch e.Name() {
		case "sources.json", "manifest.json":
			continue
		}

		data, err := os.ReadFile(filepath.Join(corpusDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}

		var c corpusCase
		if err := json.Unmarshal(data, &c); err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		cases = append(cases, c)
	}
	return cases
}

func containsString(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}
