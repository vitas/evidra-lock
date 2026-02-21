package validators

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"samebits.com/evidra-mcp/bundles/ops/schema"
)

type TrivyValidator struct{}

func (v TrivyValidator) Name() string { return "trivy" }

func (v TrivyValidator) Applicable(actionKind string) bool {
	return actionKind == "terraform.plan" || actionKind == "kustomize.build" || actionKind == "kubectl.apply"
}

func (v TrivyValidator) Run(ctx context.Context, action schema.Action, workdir string) (Report, error) {
	args := []string{"config", "--format", "json"}
	switch action.Kind {
	case "terraform.plan":
		path := payloadString(action.Payload, payloadPath)
		if path == "" {
			return Report{
				Tool:     v.Name(),
				ExitCode: 1,
				Findings: []Finding{{Tool: v.Name(), Severity: SeverityHigh, Title: "missing-path", Message: "terraform.plan requires payload.path"}},
			}, nil
		}
		args = append(args, path)
	case "kustomize.build", "kubectl.apply":
		manifestPath := payloadString(action.Payload, payloadManifestFile)
		if manifestPath == "" {
			return Report{
				Tool:     v.Name(),
				ExitCode: 1,
				Findings: []Finding{{Tool: v.Name(), Severity: SeverityHigh, Title: "missing-manifest", Message: "manifest input is required"}},
			}, nil
		}
		args = append(args, "--input", manifestPath)
	default:
		return Report{Tool: v.Name(), Findings: []Finding{}}, nil
	}

	stdout, stderr, exitCode, dur, err := execRunner(ctx, workdir, "trivy", args, nil)
	if err != nil {
		if errors.Is(err, errToolMissing) {
			rep := toolMissingReport(v.Name(), "trivy binary not found")
			rep.DurationMS = dur
			return rep, nil
		}
		return Report{}, err
	}
	rep := Report{Tool: v.Name(), ExitCode: exitCode, DurationMS: dur, Summary: map[string]interface{}{}}
	findings, parseErr := parseTrivyFindings(stdout)
	if parseErr == nil && len(findings) > 0 {
		rep.Findings = findings
		return rep, nil
	}
	if exitCode != 0 {
		rep.Findings = []Finding{{
			Tool:     v.Name(),
			Severity: SeverityHigh,
			Title:    "trivy-findings",
			Message:  strings.TrimSpace(string(stderr)),
			Raw:      strings.TrimSpace(string(stdout)),
		}}
	}
	return rep, nil
}

func parseTrivyFindings(raw []byte) ([]Finding, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	resultsRaw, ok := payload["Results"]
	if !ok {
		return nil, fmt.Errorf("missing Results")
	}
	results, ok := resultsRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid Results")
	}
	findings := make([]Finding, 0)
	for _, item := range results {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		target := payloadString(obj, "Target")
		mis, _ := obj["Misconfigurations"].([]interface{})
		for _, m := range mis {
			mi, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			sev := mapTrivySeverity(payloadString(mi, "Severity"))
			findings = append(findings, Finding{
				Tool:     "trivy",
				Severity: sev,
				Title:    payloadString(mi, "Title"),
				Message:  payloadString(mi, "Description"),
				Resource: target,
				RuleID:   payloadString(mi, "ID"),
				Raw:      mi,
			})
		}
	}
	return findings, nil
}

func mapTrivySeverity(sev string) Severity {
	switch strings.ToUpper(strings.TrimSpace(sev)) {
	case "CRITICAL":
		return SeverityCritical
	case "HIGH":
		return SeverityHigh
	case "MEDIUM":
		return SeverityMedium
	case "LOW":
		return SeverityLow
	default:
		return SeverityInfo
	}
}
