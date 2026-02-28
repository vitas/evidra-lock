# Source Attribution

Rule: argocd.dangerous_sync_combo

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: Argo CD Automated Sync semantics (prune/self-heal)
- URL: https://argo-cd.readthedocs.io/en/release-2.7/user-guide/auto_sync/
- Commit/Tag: release-2.7
- File paths:
  - user-guide/auto_sync/
- Relevant snippet: Documents prune and self-heal behavior under automated sync.

## Candidate B
- Kind: docs_example
- Title: Amazon EKS Argo CD concepts and sync options
- URL: https://docs.aws.amazon.com/eks/latest/userguide/argocd-concepts.html
- Commit/Tag: latest
- File paths:
  - argocd-concepts.html
- Relevant snippet: Documents operational sync options and project guardrails for Argo CD workloads.
