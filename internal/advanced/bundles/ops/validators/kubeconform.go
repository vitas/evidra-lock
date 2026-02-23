package validators

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"samebits.com/evidra-mcp/bundles/ops/schema"
)

type KubeconformValidator struct{}

func (v KubeconformValidator) Name() string { return "kubeconform" }

func (v KubeconformValidator) Applicable(actionKind string) bool {
	return actionKind == "kustomize.build" || actionKind == "kubectl.apply"
}

func (v KubeconformValidator) Run(ctx context.Context, action schema.Action, workdir string) (Report, error) {
	manifest := payloadString(action.Payload, payloadManifestYAML)
	if manifest == "" {
		return Report{
			Tool:     v.Name(),
			ExitCode: 1,
			Findings: []Finding{{Tool: v.Name(), Severity: SeverityHigh, Title: "missing-manifest", Message: "manifest YAML is required"}},
		}, nil
	}

	args := []string{"-strict", "-summary", "-output", "json"}
	stdout, stderr, exitCode, dur, err := execRunner(ctx, workdir, "kubeconform", args, []byte(manifest))
	if err != nil {
		if errors.Is(err, errToolMissing) {
			rep := toolMissingReport(v.Name(), "kubeconform binary not found")
			rep.DurationMS = dur
			return rep, nil
		}
		return Report{}, err
	}
	rep := Report{Tool: v.Name(), ExitCode: exitCode, DurationMS: dur, Summary: map[string]interface{}{}}

	if parsed := parseKubeconformFindings(stdout); len(parsed) > 0 {
		rep.Findings = parsed
		return rep, nil
	}
	if exitCode != 0 {
		rep.Findings = []Finding{{
			Tool:     v.Name(),
			Severity: SeverityHigh,
			Title:    "kubeconform-invalid",
			Message:  strings.TrimSpace(string(stderr)),
			Raw:      strings.TrimSpace(string(stdout)),
		}}
	}
	return rep, nil
}

func parseKubeconformFindings(raw []byte) []Finding {
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	findings := []Finding{}
	resources, _ := payload["resources"].([]interface{})
	for _, r := range resources {
		obj, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		status := strings.ToLower(payloadString(obj, "status"))
		if status == "valid" {
			continue
		}
		findings = append(findings, Finding{
			Tool:     "kubeconform",
			Severity: SeverityHigh,
			Title:    "schema-invalid",
			Message:  payloadString(obj, "msg"),
			Resource: payloadString(obj, "resource"),
			File:     payloadString(obj, "filename"),
			Raw:      obj,
		})
	}
	return findings
}
