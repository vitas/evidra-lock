package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
	opscfg "samebits.com/evidra-mcp/bundles/ops/config"
)

func Init(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ops init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	path := fs.String("path", ".", "target directory")
	force := fs.Bool("force", false, "overwrite existing files")
	enableValidators := fs.Bool("enable-validators", false, "enable built-in validators in generated config")
	withPlugins := fs.Bool("with-plugins", false, "include example exec plugin configs")
	minimal := fs.Bool("minimal", false, "write only config, no examples")
	printOnly := fs.Bool("print", false, "print generated config and do not write files")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, "usage: evidra ops init [--path dir] [--force] [--enable-validators] [--with-plugins] [--minimal] [--print]")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: evidra ops init [--path dir] [--force] [--enable-validators] [--with-plugins] [--minimal] [--print]")
		return 2
	}

	cfg := buildInitConfig(*enableValidators, *withPlugins)
	cfgRaw, err := marshalYAML(cfg)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}

	if *printOnly {
		fmt.Fprintln(stdout, string(cfgRaw))
		fmt.Fprintln(stderr, "print mode enabled: no files were written")
		return 0
	}

	root := filepath.Clean(*path)
	evidraDir := filepath.Join(root, ".evidra")
	if err := os.MkdirAll(evidraDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "create directory %s: %v\n", evidraDir, err)
		return 1
	}

	configPath := filepath.Join(evidraDir, "ops.yaml")
	if err := writeFile(configPath, cfgRaw, *force); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}

	if !*minimal {
		examplesDir := filepath.Join(evidraDir, "examples")
		if err := os.MkdirAll(examplesDir, 0o755); err != nil {
			fmt.Fprintf(stderr, "create examples directory: %v\n", err)
			return 1
		}
		for name, content := range initScenarios() {
			if err := writeFile(filepath.Join(examplesDir, name), []byte(content), *force); err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
		}
	}

	if *withPlugins {
		pluginsDir := filepath.Join(evidraDir, "plugins")
		if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
			fmt.Fprintf(stderr, "create plugins directory: %v\n", err)
			return 1
		}
		for name, content := range initPluginExamples() {
			if err := writeFile(filepath.Join(pluginsDir, name), []byte(content), *force); err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
		}
	}

	fmt.Fprintf(stdout, "Initialized Evidra Ops in %s\n", evidraDir)
	fmt.Fprintf(stdout, "Config: %s\n", configPath)
	if !*minimal {
		fmt.Fprintf(stdout, "Examples: %s\n", filepath.Join(evidraDir, "examples"))
	}
	if *withPlugins {
		fmt.Fprintf(stdout, "Plugin examples: %s\n", filepath.Join(evidraDir, "plugins"))
	}
	printNextSteps(stdout, *enableValidators, *minimal)
	return 0
}

func buildInitConfig(enableValidators, withPlugins bool) opscfg.OpsConfig {
	cfg := opscfg.Default()
	cfg.EnableValidators = enableValidators
	if !enableValidators {
		cfg.Validators.Builtins = []string{}
	}
	if withPlugins {
		cfg.Validators.ExecPlugins = []opscfg.ExecPluginConfig{
			{
				Name:            "conftest",
				Command:         "conftest",
				Args:            []string{"test", "-o", "json", "-"},
				ApplicableKinds: []string{"kustomize.build", "kubectl.apply"},
				TimeoutSeconds:  30,
			},
			{
				Name:            "checkov",
				Command:         "checkov",
				Args:            []string{"-f", "./.evidra-manifest.yaml", "-o", "json"},
				ApplicableKinds: []string{"terraform.plan", "kustomize.build", "kubectl.apply"},
				TimeoutSeconds:  30,
			},
		}
	}
	return cfg
}

func marshalYAML(cfg opscfg.OpsConfig) ([]byte, error) {
	out := map[string]interface{}{
		"enable_validators": cfg.EnableValidators,
		"validators": map[string]interface{}{
			"builtins": cfg.Validators.Builtins,
		},
		"decision": map[string]interface{}{
			"fail_on": cfg.Decision.FailOn,
		},
	}
	if len(cfg.Decision.WarnOn) > 0 {
		out["decision"].(map[string]interface{})["warn_on"] = cfg.Decision.WarnOn
	}
	if len(cfg.Validators.ExecPlugins) > 0 {
		out["validators"].(map[string]interface{})["exec_plugins"] = cfg.Validators.ExecPlugins
	}
	return yaml.Marshal(out)
}

func writeFile(path string, content []byte, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("refusing to overwrite existing file (use --force): %s", path)
		}
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func printNextSteps(stdout io.Writer, validatorsEnabled, minimal bool) {
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Next steps:")
	if !minimal {
		fmt.Fprintln(stdout, "- Validate FAIL example: evidra ops validate .evidra/examples/scenario_kubectl_apply_prod_block.json")
		fmt.Fprintln(stdout, "- Validate PASS example: evidra ops validate .evidra/examples/scenario_breakglass_audited.json")
	}
	if !validatorsEnabled {
		fmt.Fprintln(stdout, "- Enable validators by editing .evidra/ops.yaml or re-run with --enable-validators")
	}
	fmt.Fprintln(stdout, "- List validators: evidra ops validate --list-validators")
}

func initScenarios() map[string]string {
	return map[string]string{
		"scenario_kustomize_build_pass.json": `{
  "scenario_id": "sc-kustomize-pass-001",
  "actor": {"type": "human", "id": "ops-engineer-1"},
  "source": "cli",
  "timestamp": "2026-02-21T12:00:00Z",
  "actions": [
    {
      "kind": "kustomize.build",
      "target": {"namespace": "staging"},
      "intent": "build and validate manifests for staging release candidate",
      "payload": {"path": "./deploy/kustomize", "overlay": "staging"},
      "risk_tags": []
    }
  ]
}`,
		"scenario_kubectl_apply_prod_block.json": `{
  "scenario_id": "sc-kubectl-prod-block-001",
  "actor": {"type": "agent", "id": "ops-agent-42"},
  "source": "mcp",
  "timestamp": "2026-02-21T12:05:00Z",
  "actions": [
    {
      "kind": "kubectl.apply",
      "target": {"namespace": "prod"},
      "intent": "apply deployment changes to production namespace",
      "payload": {"manifests_ref": "inline", "inline_yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: app-config\n  namespace: prod"},
      "risk_tags": []
    }
  ]
}`,
		"scenario_terraform_with_validators.json": `{
  "scenario_id": "sc-terraform-validators-001",
  "actor": {"type": "agent", "id": "ops-agent-42"},
  "source": "mcp",
  "timestamp": "2026-02-21T12:10:00Z",
  "actions": [
    {
      "kind": "terraform.plan",
      "target": {"dir": "./infra"},
      "intent": "validate terraform changes with scanners before apply decision",
      "payload": {"enable_validators": true, "path": "./infra", "skip_plan": false, "publicly_exposed": false},
      "risk_tags": []
    }
  ]
}`,
		"scenario_breakglass_audited.json": `{
  "scenario_id": "sc-breakglass-001",
  "actor": {"type": "agent", "id": "ops-agent-42"},
  "source": "mcp",
  "timestamp": "2026-02-21T12:15:00Z",
  "actions": [
    {
      "kind": "k8s.apply",
      "target": {"namespace": "kube-system", "cluster": "prod-eu-1"},
      "intent": "apply emergency control-plane hotfix with incident reference",
      "payload": {"manifest": "control-plane-hotfix.yaml"},
      "risk_tags": ["breakglass", "incident-1234"]
    }
  ]
}`,
	}
}

func initPluginExamples() map[string]string {
	return map[string]string{
		"conftest.json": strings.TrimSpace(`{
  "name": "conftest",
  "command": "conftest",
  "args": ["test", "-o", "json", "-"],
  "applicable_kinds": ["kustomize.build", "kubectl.apply"],
  "timeout_seconds": 30,
  "env": {}
}`) + "\n",
		"checkov.json": strings.TrimSpace(`{
  "name": "checkov",
  "command": "checkov",
  "args": ["-f", "./.evidra-manifest.yaml", "-o", "json"],
  "applicable_kinds": ["terraform.plan", "kustomize.build", "kubectl.apply"],
  "timeout_seconds": 30,
  "env": {}
}`) + "\n",
	}
}
