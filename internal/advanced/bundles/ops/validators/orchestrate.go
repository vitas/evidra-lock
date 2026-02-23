package validators

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	opscfg "samebits.com/evidra-mcp/bundles/ops/config"
	"samebits.com/evidra-mcp/bundles/ops/schema"
)

type RunOptions struct {
	Config         opscfg.OpsConfig
	EnableOverride *bool
	BuiltinFilter  map[string]bool
	ExecMode       string
	ExecFilter     map[string]bool
}

func RunForScenario(ctx context.Context, sc schema.Scenario, workdir string, opts RunOptions) (RunResult, map[string]interface{}, error) {
	reg := Registry{}
	for _, v := range selectedBuiltins(opts.Config.Validators.Builtins, opts.BuiltinFilter) {
		reg.Register(v)
	}

	reports := make([]Report, 0)
	actionSummaries := make([]map[string]interface{}, 0, len(sc.Actions))
	anyEnabled := false

	for i, action := range sc.Actions {
		summary := map[string]interface{}{
			"index": i,
			"kind":  action.Kind,
		}
		actionEnabled := shouldRunValidators(action, opts.Config.EnableValidators, opts.EnableOverride)
		if !actionEnabled {
			summary["validators_enabled"] = false
			actionSummaries = append(actionSummaries, summary)
			continue
		}
		anyEnabled = true
		summary["validators_enabled"] = true

		enrichedAction, preReports, artifacts, err := enrichActionWithManifest(ctx, action, workdir, summary)
		if err != nil {
			reports = append(reports, Report{
				Tool:     "evidra",
				ExitCode: 1,
				Findings: []Finding{{Tool: "evidra", Severity: SeverityHigh, Title: "prepare-manifest-failed", Message: err.Error()}},
			})
			actionSummaries = append(actionSummaries, summary)
			continue
		}
		reports = append(reports, preReports...)

		for _, v := range reg.ForAction(action.Kind) {
			if (v.Name() == "kubeconform" || v.Name() == "trivy") &&
				(action.Kind == "kustomize.build" || action.Kind == "kubectl.apply") &&
				payloadString(enrichedAction.Payload, payloadManifestYAML) == "" {
				continue
			}
			rep, err := v.Run(ctx, enrichedAction, workdir)
			if err != nil {
				return RunResult{}, nil, err
			}
			reports = append(reports, rep)
			if s, ok := rep.Summary["terraform_plan_json"].(string); ok && s != "" {
				artifacts["terraform_plan_json"] = s
			}
		}
		for _, p := range selectedExecPlugins(opts.Config.Validators.ExecPlugins, opts.ExecMode, opts.ExecFilter) {
			plugin := ExecPlugin{Config: p, Scenario: sc, Artifacts: artifacts}
			if !plugin.Applicable(action.Kind) {
				continue
			}
			rep, err := plugin.Run(ctx, enrichedAction, workdir)
			if err != nil {
				return RunResult{}, nil, err
			}
			reports = append(reports, rep)
		}
		actionSummaries = append(actionSummaries, summary)
	}

	if !anyEnabled {
		return RunResult{
			Reports:   []Report{},
			Decision:  "PASS",
			RiskLevel: "normal",
			Reasons:   []string{},
		}, map[string]interface{}{"action_summaries": actionSummaries}, nil
	}

	result := Decide(sc, reports, opts.Config.Decision)
	meta := map[string]interface{}{
		"action_summaries": actionSummaries,
	}
	return result, meta, nil
}

func enrichActionWithManifest(ctx context.Context, action schema.Action, workdir string, summary map[string]interface{}) (schema.Action, []Report, map[string]string, error) {
	payload := clonePayload(action.Payload)
	preReports := []Report{}
	artifacts := map[string]string{}
	switch action.Kind {
	case "kustomize.build":
		path := payloadString(payload, payloadPath)
		if path == "" {
			return copyActionWithPayload(action, payload), preReports, artifacts, fmt.Errorf("kustomize.build requires payload.path")
		}
		buildPath := path
		if overlay := payloadString(payload, payloadOverlay); overlay != "" {
			buildPath = filepath.Join(path, overlay)
		}
		out, stderr, exitCode, _, err := execRunner(ctx, workdir, "kustomize", []string{"build", buildPath}, nil)
		if err != nil {
			if errors.Is(err, errToolMissing) {
				preReports = append(preReports, toolMissingReport("kustomize", "kustomize binary not found"))
				return copyActionWithPayload(action, payload), preReports, artifacts, nil
			}
			return copyActionWithPayload(action, payload), preReports, artifacts, err
		}
		if exitCode != 0 {
			return copyActionWithPayload(action, payload), preReports, artifacts, fmt.Errorf("kustomize build failed: %s", string(stderr))
		}
		yaml := string(out)
		payload[payloadManifestYAML] = yaml
		sum := sha256.Sum256([]byte(yaml))
		summary["manifest_sha256"] = hex.EncodeToString(sum[:])
		manifestPath, err := writeTempManifest(workdir, yaml)
		if err == nil {
			payload[payloadManifestFile] = manifestPath
			summary["manifest_temp_file"] = manifestPath
			artifacts["rendered_yaml"] = manifestPath
		}
	case "kubectl.apply":
		yaml, err := readManifestForKubectl(copyActionWithPayload(action, payload), workdir)
		if err != nil {
			return copyActionWithPayload(action, payload), preReports, artifacts, err
		}
		payload[payloadManifestYAML] = yaml
		sum := sha256.Sum256([]byte(yaml))
		summary["manifest_sha256"] = hex.EncodeToString(sum[:])
		manifestPath, err := writeTempManifest(workdir, yaml)
		if err == nil {
			payload[payloadManifestFile] = manifestPath
			summary["manifest_temp_file"] = manifestPath
			artifacts["rendered_yaml"] = manifestPath
		}
	case "terraform.plan":
		path := payloadString(payload, payloadPath)
		if path != "" {
			artifacts["terraform_path"] = path
			pj := filepath.Join(path, ".evidra.plan.json")
			if _, err := os.Stat(pj); err == nil {
				artifacts["terraform_plan_json"] = pj
			}
		}
	default:
	}
	return copyActionWithPayload(action, payload), preReports, artifacts, nil
}

func cleanupTempManifest(action schema.Action) {
	p := payloadString(action.Payload, payloadManifestFile)
	if p == "" {
		return
	}
	_ = os.Remove(p)
}

func selectedBuiltins(configured []string, filter map[string]bool) []Validator {
	factory := map[string]Validator{
		"terraform":   TerraformValidator{},
		"kubeconform": KubeconformValidator{},
		"trivy":       TrivyValidator{},
	}
	names := configured
	if len(names) == 0 {
		names = []string{"terraform", "kubeconform", "trivy"}
	}
	out := []Validator{}
	for _, n := range names {
		name := strings.ToLower(strings.TrimSpace(n))
		if len(filter) > 0 && !filter[name] {
			continue
		}
		if v, ok := factory[name]; ok {
			out = append(out, v)
		}
	}
	return out
}

func selectedExecPlugins(configured []opscfg.ExecPluginConfig, mode string, filter map[string]bool) []opscfg.ExecPluginConfig {
	out := []opscfg.ExecPluginConfig{}
	m := strings.ToLower(strings.TrimSpace(mode))
	for _, p := range configured {
		name := strings.ToLower(strings.TrimSpace(p.Name))
		switch m {
		case "none":
			continue
		case "all", "":
		default:
			if len(filter) > 0 && !filter[name] {
				continue
			}
		}
		out = append(out, p)
	}
	return out
}
