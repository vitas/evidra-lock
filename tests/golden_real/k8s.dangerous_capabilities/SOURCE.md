# Source Attribution

Rule: k8s.dangerous_capabilities

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
- Relevant snippet: Restricted/Baseline policy guidance around capability additions and privilege controls.

## Candidate B
- Kind: docs_example
- Title: Kubernetes Pod Security Admission stable announcement
- URL: https://kubernetes.io/blog/2022/08/25/pod-security-admission-stable/
- Commit/Tag: v1.25
- File paths:
  - blog/2022/08/25/pod-security-admission-stable/
- Relevant snippet: Shows admission rejections including disallowed capability patterns.
