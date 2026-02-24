package validate

import (
	"testing"

	"samebits.com/evidra/pkg/invocation"
)

// ---------------------------------------------------------------------------
// splitKind
// ---------------------------------------------------------------------------

func TestSplitKind(t *testing.T) {
	cases := []struct {
		kind      string
		wantTool  string
		wantOp    string
		wantOK    bool
	}{
		{"kubectl.apply", "kubectl", "apply", true},
		{"terraform.plan", "terraform", "plan", true},
		// SplitN(n=2) keeps everything after the first dot in the second part
		{"a.b.c", "a", "b.c", true},
		{"kubectl", "", "", false},
		{".apply", "", "", false},
		{"kubectl.", "", "", false},
		{"", "", "", false},
		{"  .  ", "", "", false},
		// outer whitespace trimmed before split, so trailing spaces on op are removed
		{"  kubectl  .  apply  ", "kubectl  ", "  apply", true},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			tool, op, ok := splitKind(tc.kind)
			if ok != tc.wantOK {
				t.Fatalf("splitKind(%q) ok=%v, want %v", tc.kind, ok, tc.wantOK)
			}
			if tool != tc.wantTool {
				t.Errorf("splitKind(%q) tool=%q, want %q", tc.kind, tool, tc.wantTool)
			}
			if op != tc.wantOp {
				t.Errorf("splitKind(%q) op=%q, want %q", tc.kind, op, tc.wantOp)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// dedupeStrings
// ---------------------------------------------------------------------------

func TestDedupeStrings(t *testing.T) {
	t.Run("removes duplicates preserving order", func(t *testing.T) {
		got := dedupeStrings([]string{"a", "b", "a", "c", "b"})
		want := []string{"a", "b", "c"}
		if !equalSlices(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("drops empty strings", func(t *testing.T) {
		got := dedupeStrings([]string{"", "a", "", "b"})
		want := []string{"a", "b"}
		if !equalSlices(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("nil input returns empty", func(t *testing.T) {
		got := dedupeStrings(nil)
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})
	t.Run("no duplicates unchanged", func(t *testing.T) {
		in := []string{"x", "y", "z"}
		got := dedupeStrings(in)
		if !equalSlices(got, in) {
			t.Errorf("got %v, want %v", got, in)
		}
	})
}

// ---------------------------------------------------------------------------
// invocationToScenario field mapping
// ---------------------------------------------------------------------------

func TestInvocationToScenario_Fields(t *testing.T) {
	inv := invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "agent", ID: "bot-1", Origin: "mcp"},
		Tool:      "kubectl",
		Operation: "apply",
		Params: map[string]interface{}{
			"target":    map[string]interface{}{"namespace": "staging"},
			"payload":   map[string]interface{}{"manifest": "deploy.yaml"},
			"risk_tags": []string{"breakglass"},
		},
		Context: map[string]interface{}{
			"source":      "ci-pipeline",
			"intent":      "deploy release",
			"scenario_id": "sc-123",
		},
	}

	sc := invocationToScenario(inv)

	if len(sc.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(sc.Actions))
	}
	a := sc.Actions[0]

	if a.Kind != "kubectl.apply" {
		t.Errorf("Kind=%q, want %q", a.Kind, "kubectl.apply")
	}
	if v, _ := a.Target["namespace"]; v != "staging" {
		t.Errorf("Target[namespace]=%v, want staging", v)
	}
	if v, _ := a.Payload["manifest"]; v != "deploy.yaml" {
		t.Errorf("Payload[manifest]=%v, want deploy.yaml", v)
	}
	if len(a.RiskTags) != 1 || a.RiskTags[0] != "breakglass" {
		t.Errorf("RiskTags=%v, want [breakglass]", a.RiskTags)
	}
	if a.Intent != "deploy release" {
		t.Errorf("Intent=%q, want %q", a.Intent, "deploy release")
	}
	if sc.Source != "ci-pipeline" {
		t.Errorf("Source=%q, want ci-pipeline", sc.Source)
	}
	if sc.Actor.Type != "agent" || sc.Actor.ID != "bot-1" {
		t.Errorf("Actor=%+v", sc.Actor)
	}
}

func TestInvocationToScenario_SourceFallback(t *testing.T) {
	inv := invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "human", ID: "u1", Origin: "cli"},
		Tool:      "git",
		Operation: "push",
		Params:    map[string]interface{}{},
		Context:   map[string]interface{}{}, // no "source" key
	}
	sc := invocationToScenario(inv)
	if sc.Source != "cli" {
		t.Errorf("Source=%q, want actor.Origin fallback %q", sc.Source, "cli")
	}
}

// ---------------------------------------------------------------------------
// scenarioIDFromInvocation priority chain
// ---------------------------------------------------------------------------

func TestScenarioIDFromInvocation_PriorityChain(t *testing.T) {
	base := invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "human", ID: "u1", Origin: "cli"},
		Tool:      "kubectl",
		Operation: "apply",
	}

	t.Run("context wins over params and generated", func(t *testing.T) {
		inv := base
		inv.Context = map[string]interface{}{"scenario_id": "ctx-id"}
		inv.Params = map[string]interface{}{"scenario_id": "param-id"}
		if got := scenarioIDFromInvocation(inv); got != "ctx-id" {
			t.Errorf("got %q, want ctx-id", got)
		}
	})

	t.Run("params win over generated when context missing", func(t *testing.T) {
		inv := base
		inv.Context = map[string]interface{}{}
		inv.Params = map[string]interface{}{"scenario_id": "param-id"}
		if got := scenarioIDFromInvocation(inv); got != "param-id" {
			t.Errorf("got %q, want param-id", got)
		}
	})

	t.Run("generated when both absent", func(t *testing.T) {
		inv := base
		inv.Context = map[string]interface{}{}
		inv.Params = map[string]interface{}{}
		got := scenarioIDFromInvocation(inv)
		if got == "" {
			t.Error("expected non-empty generated ID")
		}
		// generated format: tool.operation.<nanoseconds>
		if got == "ctx-id" || got == "param-id" {
			t.Errorf("expected generated ID, got %q", got)
		}
	})
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
