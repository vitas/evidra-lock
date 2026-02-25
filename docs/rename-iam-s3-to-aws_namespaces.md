# Rename: iam.* / s3.* → aws_iam.* / aws_s3.*

## What changed

| Old ID | New ID |
|---|---|
| `iam.wildcard_policy` | `aws_iam.wildcard_policy` |
| `iam.wildcard_principal` | `aws_iam.wildcard_principal` |
| `s3.no_encryption` | `aws_s3.no_encryption` |
| `s3.no_versioning_prod` | `aws_s3.no_versioning_prod` |
| `s3.versioning.require` (param) | `aws_s3.versioning.require` (param) |

Domain labels in documentation updated from `iam`/`s3` to `aws_iam`/`aws_s3`.

## Files renamed

| Old filename | New filename |
|---|---|
| `deny_iam_wildcard_policy.rego` | `deny_aws_iam_wildcard_policy.rego` |
| `deny_iam_wildcard_principal.rego` | `deny_aws_iam_wildcard_principal.rego` |
| `deny_s3_no_encryption.rego` | `deny_aws_s3_no_encryption.rego` |
| `deny_s3_no_versioning.rego` | `deny_aws_s3_no_versioning.rego` |
| `deny_iam_wildcard_policy_test.rego` | `deny_aws_iam_wildcard_policy_test.rego` |
| `deny_iam_wildcard_principal_test.rego` | `deny_aws_iam_wildcard_principal_test.rego` |
| `deny_s3_no_encryption_test.rego` | `deny_aws_s3_no_encryption_test.rego` |
| `deny_s3_no_versioning_test.rego` | `deny_aws_s3_no_versioning_test.rego` |

## Files modified (content)

- Data: `rule_hints/data.json` (hint keys), `params/data.json` (param key)
- Docs: `POLICY_CATALOG.md`, `ops-v0.1-baseline-rules.md`, backlog docs, `.ai` roadmap

## Why

Pre-release cleanup (pre-v0.1.0). Using `aws_iam` and `aws_s3` namespaces:

- Makes it immediately clear these rules are AWS-specific
- Distinguishes from generic IAM/S3 concepts that could apply to other cloud providers
- Avoids future namespace collisions if GCP/Azure rules are added

## Backward compatibility

Not needed. Evidra has no public release and no users. No alias or back-compat logic was added.
