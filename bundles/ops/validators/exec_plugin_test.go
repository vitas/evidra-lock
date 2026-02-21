package validators

import (
	"context"
	"testing"
	"time"

	opscfg "samebits.com/evidra-mcp/bundles/ops/config"
	"samebits.com/evidra-mcp/bundles/ops/schema"
)

func TestExecPluginRunSuccess(t *testing.T) {
	orig := execRunnerWithEnv
	defer func() { execRunnerWithEnv = orig }()
	execRunnerWithEnv = func(ctx context.Context, workdir, name string, args []string, stdin []byte, env map[string]string) ([]byte, []byte, int, int64, error) {
		return []byte(`{"tool":"demo","exit_code":0,"findings":[{"severity":"medium","title":"check","message":"ok"}],"summary":{"x":1}}`), nil, 0, 12, nil
	}
	p := ExecPlugin{
		Config: opscfg.ExecPluginConfig{
			Name:           "demo",
			Command:        "demo-plugin",
			TimeoutSeconds: 1,
		},
		Scenario: schema.Scenario{
			ScenarioID: "sc-1",
			Actor:      schema.Actor{Type: "agent"},
			Source:     "mcp",
			Timestamp:  time.Now().UTC(),
		},
	}
	rep, err := p.Run(context.Background(), schema.Action{Kind: "terraform.plan", Target: map[string]interface{}{}, Payload: map[string]interface{}{}, Intent: "valid intent text"}, ".")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if rep.Tool != "demo" || rep.ExitCode != 0 || len(rep.Findings) != 1 {
		t.Fatalf("unexpected report: %+v", rep)
	}
}

func TestExecPluginRunParseFailureFallback(t *testing.T) {
	orig := execRunnerWithEnv
	defer func() { execRunnerWithEnv = orig }()
	execRunnerWithEnv = func(ctx context.Context, workdir, name string, args []string, stdin []byte, env map[string]string) ([]byte, []byte, int, int64, error) {
		return []byte("not-json"), []byte("plugin stderr"), 1, 5, nil
	}
	p := ExecPlugin{
		Config: opscfg.ExecPluginConfig{
			Name:    "broken",
			Command: "broken-plugin",
		},
		Scenario: schema.Scenario{
			ScenarioID: "sc-1",
			Actor:      schema.Actor{Type: "human"},
			Source:     "cli",
			Timestamp:  time.Now().UTC(),
		},
	}
	rep, err := p.Run(context.Background(), schema.Action{Kind: "kustomize.build", Target: map[string]interface{}{}, Payload: map[string]interface{}{}, Intent: "valid intent text"}, ".")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(rep.Findings) != 1 || rep.Findings[0].Title != "plugin output parse error" {
		t.Fatalf("expected parse error finding, got %+v", rep.Findings)
	}
}

func TestExecPluginRunToolMissingWarning(t *testing.T) {
	orig := execRunnerWithEnv
	defer func() { execRunnerWithEnv = orig }()
	execRunnerWithEnv = func(ctx context.Context, workdir, name string, args []string, stdin []byte, env map[string]string) ([]byte, []byte, int, int64, error) {
		return nil, nil, -1, 2, errToolMissing
	}
	p := ExecPlugin{
		Config: opscfg.ExecPluginConfig{
			Name:    "missing",
			Command: "missing-plugin",
		},
		Scenario: schema.Scenario{
			ScenarioID: "sc-1",
			Actor:      schema.Actor{Type: "human"},
			Source:     "cli",
			Timestamp:  time.Now().UTC(),
		},
	}
	rep, err := p.Run(context.Background(), schema.Action{Kind: "kustomize.build", Target: map[string]interface{}{}, Payload: map[string]interface{}{}, Intent: "valid intent text"}, ".")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(rep.Findings) != 1 || rep.Findings[0].Title != "tool-missing" {
		t.Fatalf("expected tool-missing finding, got %+v", rep.Findings)
	}
}
