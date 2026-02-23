package validators

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	opscfg "samebits.com/evidra-mcp/bundles/ops/config"
	"samebits.com/evidra-mcp/bundles/ops/schema"
)

type ExecPlugin struct {
	Config    opscfg.ExecPluginConfig
	Scenario  schema.Scenario
	Artifacts map[string]string
}

func (p ExecPlugin) Name() string {
	return strings.TrimSpace(p.Config.Name)
}

func (p ExecPlugin) Applicable(actionKind string) bool {
	if len(p.Config.ApplicableKinds) == 0 {
		return true
	}
	for _, k := range p.Config.ApplicableKinds {
		if strings.TrimSpace(k) == actionKind {
			return true
		}
	}
	return false
}

func (p ExecPlugin) Run(ctx context.Context, action schema.Action, workdir string) (Report, error) {
	timeout := time.Duration(p.Config.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	in := PluginInput{
		ScenarioID: p.Scenario.ScenarioID,
		Actor:      PluginActor{Type: p.Scenario.Actor.Type},
		Source:     p.Scenario.Source,
		Timestamp:  p.Scenario.Timestamp.Format(time.RFC3339),
		Action:     action,
		Workdir:    workdir,
		Artifacts:  p.Artifacts,
		Env:        p.Config.Env,
	}
	rawIn, err := json.Marshal(in)
	if err != nil {
		return Report{}, err
	}

	stdout, stderr, processExit, dur, err := execRunnerWithEnv(runCtx, workdir, p.Config.Command, p.Config.Args, rawIn, p.Config.Env)
	if err != nil {
		if errors.Is(err, errToolMissing) {
			rep := toolMissingReport(p.Name(), fmt.Sprintf("%s binary not found", p.Config.Command))
			rep.DurationMS = dur
			return rep, nil
		}
		return Report{}, err
	}

	out := PluginOutput{}
	if parseErr := json.Unmarshal(stdout, &out); parseErr != nil {
		return Report{
			Tool:       p.Name(),
			ExitCode:   processExit,
			DurationMS: dur,
			Findings: []Finding{{
				Tool:     p.Name(),
				Severity: SeverityHigh,
				Title:    "plugin output parse error",
				Message:  fmt.Sprintf("failed to parse plugin output JSON: %v", parseErr),
				Raw: map[string]string{
					"stdout": truncateForMessage(string(stdout), 400),
					"stderr": truncateForMessage(string(stderr), 400),
				},
			}},
			Summary: map[string]interface{}{
				"stderr": truncateForMessage(string(stderr), 2000),
			},
		}, nil
	}

	tool := strings.TrimSpace(out.Tool)
	if tool == "" {
		tool = p.Name()
	}
	summary := out.Summary
	if summary == nil {
		summary = map[string]interface{}{}
	}
	if strings.TrimSpace(string(stderr)) != "" {
		summary["stderr"] = truncateForMessage(string(stderr), 2000)
	}
	exitCode := out.ExitCode
	if exitCode == 0 && processExit != 0 {
		exitCode = processExit
	}
	for i := range out.Findings {
		if out.Findings[i].Tool == "" {
			out.Findings[i].Tool = tool
		}
		if out.Findings[i].Severity == "" {
			out.Findings[i].Severity = SeverityInfo
		}
	}
	return Report{
		Tool:       tool,
		ExitCode:   exitCode,
		DurationMS: dur,
		Findings:   out.Findings,
		Summary:    summary,
	}, nil
}

func truncateForMessage(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max] + "...[truncated]"
}
