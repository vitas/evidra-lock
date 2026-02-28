# Source Attribution

Rule: k8s.hostpath_mount

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: Kubernetes hostPath volume documentation
- URL: https://kubernetes.io/docs/concepts/storage/volumes/#hostpath
- Commit/Tag: latest
- File paths:
  - docs/concepts/storage/volumes/#hostpath
- Relevant snippet: Warns about host filesystem exposure risks from hostPath volumes.

## Candidate B
- Kind: docs_example
- Title: Kubernetes Pod Security Standards
- URL: https://kubernetes.io/docs/concepts/security/pod-security-standards/
- Commit/Tag: latest
- File paths:
  - docs/concepts/security/pod-security-standards
- Relevant snippet: Restricted profile guidance includes hostPath-related hardening constraints.
