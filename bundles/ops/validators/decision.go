package validators

import (
	"fmt"
	"strings"

	opscfg "samebits.com/evidra-mcp/bundles/ops/config"
	"samebits.com/evidra-mcp/bundles/ops/schema"
)

func Decide(sc schema.Scenario, reports []Report, cfg opscfg.DecisionConfig) RunResult {
	result := RunResult{
		Reports:   reports,
		Decision:  "PASS",
		RiskLevel: "normal",
		Reasons:   []string{},
	}
	failSet := toSeveritySet(cfg.FailOn)
	if len(failSet) == 0 {
		failSet = map[Severity]bool{SeverityHigh: true, SeverityCritical: true}
	}
	warnSet := toSeveritySet(cfg.WarnOn)

	if sc.Actor.Type == "agent" && sc.Source == "mcp" {
		result.RiskLevel = "high"
		result.Reasons = append(result.Reasons, "autonomous-execution")
	}

	for _, a := range sc.Actions {
		for _, tag := range a.RiskTags {
			if tag == "breakglass" {
				result.RiskLevel = "high"
				result.Reasons = appendReason(result.Reasons, "breakglass")
			}
		}
	}

	for _, rep := range reports {
		for _, f := range rep.Findings {
			if failSet[f.Severity] {
				result.Decision = "FAIL"
				result.RiskLevel = "high"
				result.Reasons = appendReason(result.Reasons, fmt.Sprintf("%s: %s: %s", rep.Tool, strings.ToUpper(string(f.Severity)), f.Title))
				continue
			}
			if warnSet[f.Severity] {
				result.Reasons = appendReason(result.Reasons, fmt.Sprintf("%s: %s: %s", rep.Tool, strings.ToUpper(string(f.Severity)), f.Title))
			}
			if f.Title == "tool-missing" {
				result.Reasons = appendReason(result.Reasons, fmt.Sprintf("tool missing: %s", rep.Tool))
			}
		}
	}

	if len(result.Reasons) > 10 {
		result.Reasons = result.Reasons[:10]
	}
	return result
}

func toSeveritySet(levels []string) map[Severity]bool {
	out := map[Severity]bool{}
	for _, l := range levels {
		switch strings.ToLower(strings.TrimSpace(l)) {
		case string(SeverityInfo):
			out[SeverityInfo] = true
		case string(SeverityLow):
			out[SeverityLow] = true
		case string(SeverityMedium):
			out[SeverityMedium] = true
		case string(SeverityHigh):
			out[SeverityHigh] = true
		case string(SeverityCritical):
			out[SeverityCritical] = true
		}
	}
	return out
}

func appendReason(reasons []string, reason string) []string {
	for _, r := range reasons {
		if r == reason {
			return reasons
		}
	}
	return append(reasons, reason)
}
