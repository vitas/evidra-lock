# Source Attribution

Rule: k8s.run_as_root

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: Kubernetes security context task
- URL: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
- Commit/Tag: latest
- File paths:
  - docs/tasks/configure-pod-container/security-context
- Relevant snippet: Shows runAsUser and runAsNonRoot settings for containers/pods.

## Candidate B
- Kind: docs_example
- Title: Kubernetes Pod Security Standards
- URL: https://kubernetes.io/docs/concepts/security/pod-security-standards/
- Commit/Tag: latest
- File paths:
  - docs/concepts/security/pod-security-standards
- Relevant snippet: Restricted policy guidance includes non-root execution controls.
