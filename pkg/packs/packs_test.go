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
	if defs[0].BuildCommand == nil {
		t.Fatalf("expected build command to be set")
	}
}

func TestArgoCDBasicPackLoads(t *testing.T) {
	root := t.TempDir()
	writePack(t, filepath.Join(root, "argocd-basic", "pack.yaml"), `
pack: "argocd-basic"
version: "0.1"
tools:
  - name: "argocd"
    kind: "cli"
    binary: "argocd"
    operations:
      - name: "version"
        args: ["version"]
        params: {}
      - name: "app-sync"
        args: ["app", "sync", "{{app}}"]
        params:
          app: {type: "string", required: true}
`)

	defs, err := LoadToolDefinitions(root, []string{"kubectl"})
	if err != nil {
		t.Fatalf("LoadToolDefinitions() error = %v", err)
	}
	if len(defs) != 1 || defs[0].Name != "argocd" {
		t.Fatalf("expected argocd definition, got %#v", defs)
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
	_, err := registry.BuildDeclarativeCLIArgs(spec.Binary, spec, "get", map[string]interface{}{})
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
	argv, err := registry.BuildDeclarativeCLIArgs(spec.Binary, spec, "post", map[string]interface{}{
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

func TestHelmBasicPackLoadsAndValidates(t *testing.T) {
	root := t.TempDir()
	writePack(t, filepath.Join(root, "helm-basic", "pack.yaml"), `
pack: "helm-basic"
version: "0.1"
tools:
  - name: "helm"
    kind: "cli"
    binary: "helm"
    operations:
      - name: "version"
        args: ["version"]
        params: {}
      - name: "status"
        args: ["status", "{{release}}", "--namespace", "{{namespace}}"]
        params:
          release: {type: "string", required: true}
          namespace: {type: "string", required: true}
      - name: "upgrade"
        args: ["upgrade", "{{release}}", "{{chart}}", "--namespace", "{{namespace}}", "--install"]
        params:
          release: {type: "string", required: true}
          chart: {type: "string", required: true}
          namespace: {type: "string", required: true}
`)

	defs, err := LoadToolDefinitions(root, []string{"echo", "git", "kubectl"})
	if err != nil {
		t.Fatalf("LoadToolDefinitions() error = %v", err)
	}
	if len(defs) != 1 || defs[0].Name != "helm" {
		t.Fatalf("expected helm tool definition, got %#v", defs)
	}

	if err := defs[0].ValidateParams("status", map[string]interface{}{"release": "r1"}); err == nil {
		t.Fatalf("expected missing namespace validation error")
	}
	if err := defs[0].ValidateParams("status", map[string]interface{}{"release": "r1", "namespace": "ns1"}); err != nil {
		t.Fatalf("expected status validation success, got %v", err)
	}
}

func TestHelmUpgradeArgvStableOutput(t *testing.T) {
	spec := registry.CLIToolSpec{
		Binary: "helm",
		Operations: map[string]registry.CLIOperationSpec{
			"upgrade": {
				Args: []string{"upgrade", "{{release}}", "{{chart}}", "--namespace", "{{namespace}}", "--install"},
				Params: map[string]registry.ParamRule{
					"release":   {Type: "string", Required: true},
					"chart":     {Type: "string", Required: true},
					"namespace": {Type: "string", Required: true},
				},
			},
		},
	}

	argv, err := registry.BuildDeclarativeCLIArgs(spec.Binary, spec, "upgrade", map[string]interface{}{
		"release":   "payments-api",
		"chart":     "./charts/payments-api",
		"namespace": "payments",
	})
	if err != nil {
		t.Fatalf("BuildDeclarativeCLIArgs() error = %v", err)
	}
	want := []string{"helm", "upgrade", "payments-api", "./charts/payments-api", "--namespace", "payments", "--install"}
	if len(argv) != len(want) {
		t.Fatalf("argv len mismatch: got %v want %v", argv, want)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Fatalf("argv mismatch at %d: got %q want %q", i, argv[i], want[i])
		}
	}
}

func TestArgoCDAppSyncArgvAndValidation(t *testing.T) {
	spec := registry.CLIToolSpec{
		Binary: "argocd",
		Operations: map[string]registry.CLIOperationSpec{
			"app-sync": {
				Args: []string{"app", "sync", "{{app}}"},
				Params: map[string]registry.ParamRule{
					"app": {Type: "string", Required: true},
				},
			},
		},
	}

	if _, err := registry.BuildDeclarativeCLIArgs(spec.Binary, spec, "app-sync", map[string]interface{}{}); err == nil {
		t.Fatalf("expected required param validation error for app")
	}

	argv, err := registry.BuildDeclarativeCLIArgs(spec.Binary, spec, "app-sync", map[string]interface{}{
		"app": "payments-api",
	})
	if err != nil {
		t.Fatalf("BuildDeclarativeCLIArgs() error = %v", err)
	}
	want := []string{"argocd", "app", "sync", "payments-api"}
	if len(argv) != len(want) {
		t.Fatalf("argv len mismatch: got %v want %v", argv, want)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Fatalf("argv mismatch at %d: got %q want %q", i, argv[i], want[i])
		}
	}
}

func TestTerraformBasicPackLoads(t *testing.T) {
	root := t.TempDir()
	writePack(t, filepath.Join(root, "terraform-basic", "pack.yaml"), `
pack: "terraform-basic"
version: "0.1"
tools:
  - name: "terraform"
    kind: "cli"
    binary: "terraform"
    operations:
      - name: "plan"
        args: ["-chdir={{dir}}", "plan"]
        params:
          dir: {type: "string", required: true}
      - name: "apply"
        args: ["-chdir={{dir}}", "apply", "-auto-approve"]
        params:
          dir: {type: "string", required: true}
`)

	defs, err := LoadToolDefinitions(root, []string{"kubectl"})
	if err != nil {
		t.Fatalf("LoadToolDefinitions() error = %v", err)
	}
	if len(defs) != 1 || defs[0].Name != "terraform" {
		t.Fatalf("expected terraform definition, got %#v", defs)
	}
}

func TestTerraformPlanApplyValidationAndArgv(t *testing.T) {
	spec := registry.CLIToolSpec{
		Binary: "terraform",
		Operations: map[string]registry.CLIOperationSpec{
			"plan": {
				Args: []string{"-chdir={{dir}}", "plan"},
				Params: map[string]registry.ParamRule{
					"dir": {Type: "string", Required: true},
				},
			},
			"apply": {
				Args: []string{"-chdir={{dir}}", "apply", "-auto-approve"},
				Params: map[string]registry.ParamRule{
					"dir": {Type: "string", Required: true},
				},
			},
		},
	}

	if _, err := registry.BuildDeclarativeCLIArgs(spec.Binary, spec, "plan", map[string]interface{}{}); err == nil {
		t.Fatalf("expected required dir validation error for plan")
	}
	if _, err := registry.BuildDeclarativeCLIArgs(spec.Binary, spec, "apply", map[string]interface{}{}); err == nil {
		t.Fatalf("expected required dir validation error for apply")
	}

	planArgv, err := registry.BuildDeclarativeCLIArgs(spec.Binary, spec, "plan", map[string]interface{}{"dir": "./infra"})
	if err != nil {
		t.Fatalf("plan argv build error = %v", err)
	}
	wantPlan := []string{"terraform", "-chdir=./infra", "plan"}
	if len(planArgv) != len(wantPlan) {
		t.Fatalf("plan argv len mismatch: got %v want %v", planArgv, wantPlan)
	}
	for i := range wantPlan {
		if planArgv[i] != wantPlan[i] {
			t.Fatalf("plan argv mismatch at %d: got %q want %q", i, planArgv[i], wantPlan[i])
		}
	}

	applyArgv, err := registry.BuildDeclarativeCLIArgs(spec.Binary, spec, "apply", map[string]interface{}{"dir": "./infra"})
	if err != nil {
		t.Fatalf("apply argv build error = %v", err)
	}
	wantApply := []string{"terraform", "-chdir=./infra", "apply", "-auto-approve"}
	if len(applyArgv) != len(wantApply) {
		t.Fatalf("apply argv len mismatch: got %v want %v", applyArgv, wantApply)
	}
	for i := range wantApply {
		if applyArgv[i] != wantApply[i] {
			t.Fatalf("apply argv mismatch at %d: got %q want %q", i, applyArgv[i], wantApply[i])
		}
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
