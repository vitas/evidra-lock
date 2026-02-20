package packs

import (
	"os"
	"path/filepath"
	"testing"

	"samebits.com/evidra-mcp/pkg/registry"
)

func TestLoadToolDefinitionsParsesPackAndBuildsDefs(t *testing.T) {
	root := t.TempDir()
	writePack(t, filepath.Join(root, "curl-basic", "pack.yaml"), `
pack: "curl-basic"
version: "0.1"
tools:
  - name: "curl"
    kind: "cli"
    binary: "curl"
    operations:
      - name: "version"
        args: ["--version"]
        params: {}
      - name: "get"
        args: ["-sS", "{{url}}"]
        params:
          url: {type: "string", required: true}
`)

	defs, err := LoadToolDefinitions(root, []string{"echo", "git", "kubectl"})
	if err != nil {
		t.Fatalf("LoadToolDefinitions() error = %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 tool definition, got %d", len(defs))
	}
	if defs[0].Name != "curl" {
		t.Fatalf("expected curl tool, got %q", defs[0].Name)
	}
	if _, ok := defs[0].Executor, true; !ok {
		t.Fatalf("expected executor to be set")
	}
}

func TestPlaceholderValidationFailsWhenRequiredMissing(t *testing.T) {
	spec := registry.CLIToolSpec{
		Binary: "curl",
		Operations: map[string]registry.CLIOperationSpec{
			"get": {
				Args: []string{"-sS", "{{url}}"},
				Params: map[string]registry.ParamRule{
					"url": {Type: "string", Required: true},
				},
			},
		},
	}
	_, err := registry.BuildDeclarativeCLIArgs(spec, "get", map[string]interface{}{})
	if err == nil {
		t.Fatalf("expected missing required param error")
	}
}

func TestOptionalPlaceholderIsOmitted(t *testing.T) {
	spec := registry.CLIToolSpec{
		Binary: "curl",
		Operations: map[string]registry.CLIOperationSpec{
			"post": {
				Args: []string{"-sS", "-H", "{{content_type?}}", "-d", "{{data}}", "{{url}}"},
				Params: map[string]registry.ParamRule{
					"content_type": {Type: "string", Required: false},
					"data":         {Type: "string", Required: true},
					"url":          {Type: "string", Required: true},
				},
			},
		},
	}
	argv, err := registry.BuildDeclarativeCLIArgs(spec, "post", map[string]interface{}{
		"data": "{}",
		"url":  "https://example.com",
	})
	if err != nil {
		t.Fatalf("BuildDeclarativeCLIArgs() error = %v", err)
	}
	for _, arg := range argv {
		if arg == "{{content_type?}}" || arg == "" {
			t.Fatalf("expected optional placeholder to be omitted, argv=%v", argv)
		}
	}
}

func TestLoadToolDefinitionsRejectsDuplicateWithBuiltins(t *testing.T) {
	root := t.TempDir()
	writePack(t, filepath.Join(root, "dup", "pack.yaml"), `
pack: "dup"
version: "0.1"
tools:
  - name: "echo"
    kind: "cli"
    binary: "echo"
    operations:
      - name: "run"
        args: ["{{text}}"]
        params:
          text: {type: "string", required: true}
`)

	_, err := LoadToolDefinitions(root, []string{"echo", "git", "kubectl"})
	if err == nil {
		t.Fatalf("expected duplicate tool error")
	}
}

func writePack(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
