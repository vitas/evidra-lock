# debug-basic (contrib)

Optional debug pack with version and identity checks:
- `terraform-debug/version`
- `helm-debug/version`
- `argocd-debug/version`
- `aws-debug/sts-whoami`

This pack is not loaded by ops profile defaults.

To load it explicitly:

```bash
EVIDRA_PACKS_DIR=./packs/_contrib
```

