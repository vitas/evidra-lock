# Source Attribution

Rule: k8s.protected_namespace

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: Kubernetes namespace fundamentals
- URL: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/#initial-namespaces
- Commit/Tag: latest
- File paths:
  - docs/concepts/overview/working-with-objects/namespaces/#initial-namespaces
- Relevant snippet: Identifies kube-system as a special namespace for cluster components.

## Candidate B
- Kind: docs_example
- Title: Kubernetes cluster-level Pod Security Admission tutorial
- URL: https://kubernetes.io/docs/tutorials/security/cluster-level-pss/
- Commit/Tag: latest
- File paths:
  - docs/tutorials/security/cluster-level-pss
- Relevant snippet: Demonstrates exempting system namespaces from broad policy changes.
