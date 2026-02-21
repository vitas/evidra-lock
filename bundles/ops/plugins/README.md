# Ops Exec Plugins

Evidra Ops supports external validator plugins executed as local commands.

## How it works

1. Evidra sends `PluginInput` JSON to plugin stdin.
2. Plugin writes `PluginOutput` JSON to stdout.
3. Evidra normalizes output into internal `Report` + `Finding`.

Plugin process non-zero exit codes are allowed; orchestration still parses stdout when possible.

## Minimal Plugin Script Contract

Input JSON fields:

- `scenario_id`
- `actor.type`
- `source`
- `timestamp`
- `action`
- `workdir`
- `artifacts` (optional)
- `env` (optional)

Output JSON fields:

- `tool`
- `exit_code`
- `findings`
- `summary`

## Configure Plugins

Use `.evidra/ops.yaml`:

```yaml
enable_validators: true
validators:
  builtins: [terraform, kubeconform, trivy]
  exec_plugins:
    - name: conftest
      command: conftest
      args: ["test", "-o", "json", "-"]
      applicable_kinds: ["kustomize.build", "kubectl.apply"]
      timeout_seconds: 30
```

Optional examples are available in `bundles/ops/plugins/examples/`.
