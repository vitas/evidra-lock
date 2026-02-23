package validators

import (
	"testing"
	"time"

	opscfg "samebits.com/evidra-mcp/bundles/ops/config"
	"samebits.com/evidra-mcp/bundles/ops/schema"
)

func TestDecideFailsOnHighOrCritical(t *testing.T) {
	sc := baseScenarioForDecision()
	reports := []Report{
		{Tool: "trivy", Findings: []Finding{{Tool: "trivy", Severity: SeverityHigh, Title: "bad-config"}}},
	}
	got := Decide(sc, reports, opscfg.DecisionConfig{FailOn: []string{"high", "critical"}})
	if got.Decision != "FAIL" {
		t.Fatalf("expected FAIL, got %s", got.Decision)
	}
	if got.RiskLevel != "high" {
		t.Fatalf("expected high risk, got %s", got.RiskLevel)
	}
}

func TestDecidePassesWithoutHighCritical(t *testing.T) {
	sc := baseScenarioForDecision()
	sc.Actor.Type = "human"
	sc.Source = "cli"
	reports := []Report{
		{Tool: "trivy", Findings: []Finding{{Tool: "trivy", Severity: SeverityMedium, Title: "review"}}},
	}
	got := Decide(sc, reports, opscfg.DecisionConfig{FailOn: []string{"high", "critical"}})
	if got.Decision != "PASS" {
		t.Fatalf("expected PASS, got %s", got.Decision)
	}
}

func TestDecideIncludesToolMissingWarning(t *testing.T) {
	sc := baseScenarioForDecision()
	sc.Actor.Type = "human"
	sc.Source = "cli"
	reports := []Report{
		toolMissingReport("trivy", "trivy binary not found"),
	}
	got := Decide(sc, reports, opscfg.DecisionConfig{FailOn: []string{"high", "critical"}})
	if got.Decision != "PASS" {
		t.Fatalf("expected PASS when tool missing only, got %s", got.Decision)
	}
	found := false
	for _, reason := range got.Reasons {
		if reason == "tool missing: trivy" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected tool missing reason, got %v", got.Reasons)
	}
}

func baseScenarioForDecision() schema.Scenario {
	return schema.Scenario{
		ScenarioID: "sc-1",
		Actor:      schema.Actor{Type: "agent", ID: "a1"},
		Source:     "mcp",
		Timestamp:  time.Now().UTC(),
		Actions: []schema.Action{
			{Kind: "terraform.plan", Target: map[string]interface{}{"dir": "."}, Intent: "plan safe infrastructure changes", Payload: map[string]interface{}{}, RiskTags: nil},
		},
	}
}
