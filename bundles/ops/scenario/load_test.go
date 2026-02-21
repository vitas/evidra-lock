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
