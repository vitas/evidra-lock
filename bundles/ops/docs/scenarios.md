# Ops Scenario Validator Fields

External validators are optional and action-scoped.

## Global Configuration

Default config path: `.evidra/ops.yaml` (override with `--config`).

Example:

```yaml
enable_validators: true
validators:
  builtins: [terraform, kubeconform, trivy]
  exec_plugins:
    - name: conftest
      command: conftest
      args: ["test", "-o", "json", "-"]
      applicable_kinds: ["kustomize.build", "kubectl.apply"]
decision:
  fail_on: [high, critical]
  warn_on: [medium]
```

## Common Toggle

- `payload.enable_validators` (bool, default `false`)

When `true`, Evidra runs external validator wrappers for supported action kinds.

## Terraform (`terraform.plan`)

Payload fields:

- `path` (string, required for validator execution): terraform workspace directory.
- `skip_plan` (bool, optional): skip `terraform plan` and `terraform show -json`.
- `enable_validators` (bool, optional): enables external validators.

Validators used:

- `terraform validate`
- `terraform plan` + `terraform show -json` (unless `skip_plan=true`)
- `trivy config --format json <path>`

## Kustomize (`kustomize.build`)

Payload fields:

- `path` (string, required): kustomize base path.
- `overlay` (string, optional): appended under `path`.
- `enable_validators` (bool, optional): enables external validators.

Pipeline:

1. `kustomize build` runs once.
2. Generated YAML is fed into `kubeconform`.
3. Same YAML is scanned by `trivy config --input`.

## Kubectl Apply (`kubectl.apply`)

Payload fields:

- `enable_validators` (bool, optional)
- `manifests_ref`:
  - `"inline"` to use `inline_yaml`
  - otherwise treated as a file path to YAML
- `inline_yaml` (string, required when `manifests_ref=inline`)

Pipeline:

1. Resolve YAML from inline content or file.
2. Feed YAML to `kubeconform`.
3. Feed same YAML to `trivy config --input`.

## Plugin Protocol

Exec plugins receive `PluginInput` JSON on stdin and must return `PluginOutput` JSON on stdout.

Use this for tools such as `conftest`, `checkov`, `kubescore`, or internal validators without changing orchestration code.
