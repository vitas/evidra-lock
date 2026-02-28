# Source Attribution

Rule: k8s.host_namespace_escape

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: Kubernetes Pod Security Standards
- URL: https://kubernetes.io/docs/concepts/security/pod-security-standards/
- Commit/Tag: latest
- File paths:
  - docs/concepts/security/pod-security-standards
- Relevant snippet: Baseline/Restricted controls disallow host namespaces (hostNetwork/hostPID/hostIPC).

## Candidate B
- Kind: docs_example
- Title: Kubernetes Linux kernel security constraints
- URL: https://kubernetes.io/docs/concepts/security/linux-kernel-security-constraints/
- Commit/Tag: latest
- File paths:
  - docs/concepts/security/linux-kernel-security-constraints
- Relevant snippet: Explains container isolation boundaries and implications of elevated namespace access.
