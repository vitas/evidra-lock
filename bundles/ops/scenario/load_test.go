package scenario

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileStrictDecodeUnknownField(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "scenario.json")
	content := `{
  "scenario_id":"sc-1",
  "actor":{"type":"agent"},
  "source":"mcp",
  "timestamp":"2026-02-21T00:00:00Z",
  "actions":[{"kind":"terraform.plan","target":{},"intent":"safe intent text","payload":{},"risk_tags":[]}],
  "unexpected":"boom"
}`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := LoadFile(tmp); err == nil {
		t.Fatalf("LoadFile() expected unknown field error")
	}
}

func TestLoadFileValidationRequiresScenarioIDAndActions(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "scenario.json")
	content := `{
  "scenario_id":"",
  "actor":{"type":"agent"},
  "source":"mcp",
  "timestamp":"2026-02-21T00:00:00Z",
  "actions":[]
}`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := LoadFile(tmp); err == nil {
		t.Fatalf("LoadFile() expected validation error")
	}
}

func TestLoadFileDetectsTerraformPlan(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "plan.json")
	content := `{
  "resource_changes": [
    {"change": {"actions": ["create"]}}
  ],
  "planned_values": {},
  "configuration": {}
}`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write plan file: %v", err)
	}
	sc, err := LoadFile(tmp)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}
	if len(sc.Actions) != 1 {
		t.Fatalf("expected one action, got %d", len(sc.Actions))
	}
	if sc.Actions[0].Kind != "terraform.plan" {
		t.Fatalf("expected terraform.plan action, got %s", sc.Actions[0].Kind)
	}
}

func TestLoadFileDetectsKubernetesManifest(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "manifest.yaml")
	content := `apiVersion: v1
kind: ConfigMap
metadata:
  name: example
  namespace: prod
`
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write manifest file: %v", err)
	}
	sc, err := LoadFile(tmp)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}
	if len(sc.Actions) != 1 {
		t.Fatalf("expected one action, got %d", len(sc.Actions))
	}
	if sc.Actions[0].Kind != "kubectl.apply" {
		t.Fatalf("expected kubectl.apply action, got %s", sc.Actions[0].Kind)
	}
}

func TestLoadFileUnsupportedInput(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "text.txt")
	if err := os.WriteFile(tmp, []byte("plain text"), 0o644); err != nil {
		t.Fatalf("failed to write text file: %v", err)
	}
	if _, err := LoadFile(tmp); err == nil {
		t.Fatalf("expected unsupported input error")
	}
}
