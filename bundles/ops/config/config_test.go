package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsWhenFileMissing(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.EnableValidators {
		t.Fatalf("expected default enable_validators=false")
	}
	if len(cfg.Validators.Builtins) == 0 {
		t.Fatalf("expected default builtins")
	}
	if len(cfg.Decision.FailOn) == 0 {
		t.Fatalf("expected default fail_on")
	}
}

func TestLoadAppliesExecPluginTimeoutDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ops.yaml")
	raw := []byte(`
enable_validators: true
validators:
  builtins: [trivy]
  exec_plugins:
    - name: demo
      command: ./demo-plugin
      applicable_kinds: [terraform.plan]
decision:
  fail_on: [high, critical]
`)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.EnableValidators {
		t.Fatalf("expected enable_validators=true")
	}
	if len(cfg.Validators.ExecPlugins) != 1 {
		t.Fatalf("expected one exec plugin")
	}
	if cfg.Validators.ExecPlugins[0].TimeoutSeconds != 30 {
		t.Fatalf("expected default timeout_seconds=30, got %d", cfg.Validators.ExecPlugins[0].TimeoutSeconds)
	}
}
