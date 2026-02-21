package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesDirsAndFiles(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Init([]string{"--path", root}, &out, &errOut)
	if code != 0 {
		t.Fatalf("Init() code=%d stderr=%s", code, errOut.String())
	}
	mustExist(t, filepath.Join(root, ".evidra", "ops.yaml"))
	mustExist(t, filepath.Join(root, ".evidra", "examples", "scenario_breakglass_audited.json"))
}

func TestInitRespectsForce(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, ".evidra", "ops.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Init([]string{"--path", root}, &out, &errOut)
	if code == 0 {
		t.Fatalf("expected failure without --force")
	}

	out.Reset()
	errOut.Reset()
	code = Init([]string{"--path", root, "--force"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected success with --force, code=%d stderr=%s", code, errOut.String())
	}
}

func TestInitPrintDoesNotWriteFiles(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Init([]string{"--path", root, "--print"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("Init() code=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "enable_validators") {
		t.Fatalf("expected config output, got: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".evidra", "ops.yaml")); err == nil {
		t.Fatalf("expected no files written in --print mode")
	}
}

func TestInitMinimalSkipsExamples(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Init([]string{"--path", root, "--minimal"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("Init() code=%d stderr=%s", code, errOut.String())
	}
	mustExist(t, filepath.Join(root, ".evidra", "ops.yaml"))
	if _, err := os.Stat(filepath.Join(root, ".evidra", "examples")); err == nil {
		t.Fatalf("expected no examples directory with --minimal")
	}
}

func TestInitWithPluginsCreatesPluginConfigs(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Init([]string{"--path", root, "--with-plugins"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("Init() code=%d stderr=%s", code, errOut.String())
	}
	mustExist(t, filepath.Join(root, ".evidra", "plugins", "conftest.json"))
	mustExist(t, filepath.Join(root, ".evidra", "plugins", "checkov.json"))
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %s err=%v", path, err)
	}
}
