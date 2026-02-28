# Evidra — Kind Vocabulary

Every tool invocation uses a `kind` field in format `tool.operation`.

## Canonical prefixes

| prefix | tool | examples |
|---|---|---|
| `kubectl.` | Kubernetes CLI | kubectl.apply, kubectl.delete, kubectl.get |
| `terraform.` | Terraform | terraform.apply, terraform.destroy, terraform.plan |
| `helm.` | Helm | helm.upgrade, helm.uninstall, helm.list |
| `argocd.` | Argo CD | argocd.sync, argocd.project |

## Rules

- Always use canonical prefix. No aliases (`k8s.`, `tf.`, `argo.`).
- Format: `prefix.operation` (one dot, two parts).
- Adding a new prefix requires updating `allowedPrefixes` in
  `TestKindVocabulary` and `TestParamsVocabulary`.
- Destructive operations must be added to `ops.destructive_operations`
  in params/data.json.
- Read-only operations (get, list, plan, describe, show, diff, status,
  version) are auto-allowed for known prefixes.
