package validators

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"samebits.com/evidra-mcp/bundles/ops/schema"
)

type TerraformValidator struct{}

func (v TerraformValidator) Name() string { return "terraform" }

func (v TerraformValidator) Applicable(actionKind string) bool {
	return actionKind == "terraform.plan"
}

func (v TerraformValidator) Run(ctx context.Context, action schema.Action, workdir string) (Report, error) {
	path := payloadString(action.Payload, payloadPath)
	if path == "" {
		return Report{
			Tool:     v.Name(),
			ExitCode: 1,
			Findings: []Finding{{Tool: v.Name(), Severity: SeverityHigh, Title: "missing-path", Message: "terraform.plan requires payload.path"}},
		}, nil
	}

	planPath := filepath.Join(path, ".evidra.plan")
	report := Report{Tool: v.Name(), Summary: map[string]interface{}{}}

	stdout, stderr, code, dur, err := execRunner(ctx, path, "terraform", []string{"validate"}, nil)
	report.DurationMS += dur
	if err != nil {
		if errors.Is(err, errToolMissing) {
			rep := toolMissingReport(v.Name(), "terraform binary not found")
			rep.DurationMS = report.DurationMS
			return rep, nil
		}
		return Report{}, err
	}
	report.ExitCode = code
	if code != 0 {
		report.Findings = append(report.Findings, Finding{
			Tool:     v.Name(),
			Severity: SeverityHigh,
			Title:    "terraform-validate-failed",
			Message:  strings.TrimSpace(string(stderr)),
			Raw:      strings.TrimSpace(string(stdout)),
		})
	}

	if !payloadBool(action.Payload, payloadSkipPlan) {
		stdout, stderr, code, dur, err = execRunner(ctx, path, "terraform", []string{"plan", "-out=./.evidra.plan"}, nil)
		report.DurationMS += dur
		if err != nil {
			if errors.Is(err, errToolMissing) {
				rep := toolMissingReport(v.Name(), "terraform binary not found")
				rep.DurationMS = report.DurationMS
				return rep, nil
			}
			return Report{}, err
		}
		if code != 0 {
			report.ExitCode = code
			report.Findings = append(report.Findings, Finding{
				Tool:     v.Name(),
				Severity: SeverityHigh,
				Title:    "terraform-plan-failed",
				Message:  strings.TrimSpace(string(stderr)),
				Raw:      strings.TrimSpace(string(stdout)),
			})
		}

		showOut, showErr, showCode, showDur, err := execRunner(ctx, path, "terraform", []string{"show", "-json", "./.evidra.plan"}, nil)
		report.DurationMS += showDur
		if err != nil {
			if !errors.Is(err, errToolMissing) {
				return Report{}, err
			}
		} else {
			if showCode != 0 {
				report.ExitCode = showCode
				report.Findings = append(report.Findings, Finding{
					Tool:     v.Name(),
					Severity: SeverityHigh,
					Title:    "terraform-show-failed",
					Message:  strings.TrimSpace(string(showErr)),
					Raw:      strings.TrimSpace(string(showOut)),
				})
			} else {
				addTerraformSummary(report.Summary, showOut)
				planJSONPath := filepath.Join(path, ".evidra.plan.json")
				if err := os.WriteFile(planJSONPath, showOut, 0o600); err == nil {
					report.Summary["terraform_plan_json"] = planJSONPath
				}
			}
		}
	}

	report.Summary["path"] = path
	report.Summary["plan_file"] = planPath
	return report, nil
}

func addTerraformSummary(summary map[string]interface{}, showOut []byte) {
	var payload map[string]interface{}
	if err := json.Unmarshal(showOut, &payload); err != nil {
		return
	}
	changes, ok := payload["resource_changes"].([]interface{})
	if !ok {
		return
	}
	total := len(changes)
	destroyCount := 0
	for _, c := range changes {
		obj, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		changeObj, ok := obj["change"].(map[string]interface{})
		if !ok {
			continue
		}
		actions, ok := changeObj["actions"].([]interface{})
		if !ok {
			continue
		}
		for _, a := range actions {
			if s, _ := a.(string); s == "delete" {
				destroyCount++
				break
			}
		}
	}
	summary["resource_changes_total"] = total
	summary["destroy_count"] = destroyCount
}
