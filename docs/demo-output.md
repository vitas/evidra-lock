# Demo Expected Output

These are the expected outputs from the three demo scenarios in `examples/demo/`.

---

## 1. Kubernetes: Delete pods in kube-system (DENY)

```bash
evidra validate examples/demo/kubernetes_kube_system_delete.json
```

**Default output:**

```
Decision: FAIL
Risk level: high
Evidence: evt-…
Reason: Changes in restricted namespace require breakglass
Rule IDs:
- k8s.protected_namespace
Reason:
- Changes in restricted namespace require breakglass
How to fix:
- Add risk_tag: breakglass
- Or apply changes outside kube-system
```

**JSON output (`--json`):**

```json
{
  "status": "FAIL",
  "risk_level": "high",
  "reason": "Changes in restricted namespace require breakglass",
  "reasons": [
    "Changes in restricted namespace require breakglass"
  ],
  "rule_ids": [
    "k8s.protected_namespace"
  ],
  "hints": [
    "Add risk_tag: breakglass",
    "Or apply changes outside kube-system"
  ],
  "evidence_id": "evt-…",
  "timestamp": "…"
}
```

**Exit code:** `2`

---

## 2. Terraform: S3 bucket without public access block (DENY)

```bash
evidra validate examples/demo/terraform_public_s3.json
```

**Default output:**

```
Decision: FAIL
Risk level: high
Evidence: evt-…
Reason: S3 bucket missing complete Block Public Access configuration
Rule IDs:
- aws_s3.no_encryption
- terraform.s3_public_access
Reason:
- S3 bucket missing complete Block Public Access configuration
- S3 bucket missing server-side encryption
How to fix:
- Add risk_tag: approved_public if bucket must be public.
- Enable all four S3 Block Public Access settings.
- Enable server-side encryption (SSE-S3 or SSE-KMS) on the bucket.
- SSE-S3 is zero-cost and zero-performance-impact.
```

**JSON output (`--json`):**

```json
{
  "status": "FAIL",
  "risk_level": "high",
  "reason": "S3 bucket missing complete Block Public Access configuration",
  "reasons": [
    "S3 bucket missing complete Block Public Access configuration",
    "S3 bucket missing server-side encryption"
  ],
  "rule_ids": [
    "aws_s3.no_encryption",
    "terraform.s3_public_access"
  ],
  "hints": [
    "Add risk_tag: approved_public if bucket must be public.",
    "Enable all four S3 Block Public Access settings.",
    "Enable server-side encryption (SSE-S3 or SSE-KMS) on the bucket.",
    "SSE-S3 is zero-cost and zero-performance-impact."
  ],
  "evidence_id": "evt-…",
  "timestamp": "…"
}
```

**Exit code:** `2`

This scenario triggers two rules simultaneously — the S3 bucket has neither public access block configuration nor server-side encryption.

---

## 3. Kubernetes: Deploy to default namespace (PASS)

```bash
evidra validate examples/demo/kubernetes_safe_apply.json
```

**Default output:**

```
Decision: PASS
Risk level: low
Evidence: evt-…
Reason: decision unavailable
No deny rules matched.
```

**JSON output (`--json`):**

```json
{
  "status": "PASS",
  "risk_level": "low",
  "reason": "decision unavailable",
  "evidence_id": "evt-…",
  "timestamp": "…"
}
```

**Exit code:** `0`

---

## Notes

- `evt-…` is a unique evidence ID generated at evaluation time.
- Every decision (PASS or FAIL) is recorded in the evidence chain at `~/.evidra/evidence`.
- The `--json` flag produces machine-readable output for CI pipelines and scripts.
- Exit codes: `0` = PASS, `2` = FAIL, `1` = error.
