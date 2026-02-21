package validators

import "samebits.com/evidra-mcp/bundles/ops/schema"

// Exec plugin protocol (v0.1):
// - Evidra writes PluginInput as JSON to plugin stdin.
// - Plugin writes PluginOutput JSON to stdout.
// - Plugin stderr is treated as diagnostic and captured into Report.Summary["stderr"].
// - A non-zero process exit code does not bypass stdout parsing.
// - PluginOutput fields are normalized into Evidra Report/Finding.
type PluginInput struct {
	ScenarioID string            `json:"scenario_id"`
	Actor      PluginActor       `json:"actor"`
	Source     string            `json:"source"`
	Timestamp  string            `json:"timestamp"`
	Action     schema.Action     `json:"action"`
	Workdir    string            `json:"workdir"`
	Artifacts  map[string]string `json:"artifacts,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
}

type PluginActor struct {
	Type string `json:"type"`
}

type PluginOutput struct {
	Tool     string                 `json:"tool"`
	ExitCode int                    `json:"exit_code"`
	Findings []Finding              `json:"findings"`
	Summary  map[string]interface{} `json:"summary,omitempty"`
}
