# Source Attribution

Rule: argocd.autosync_prod

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: Argo CD Automated Sync Policy
- URL: https://argo-cd.readthedocs.io/en/release-2.7/user-guide/auto_sync/
- Commit/Tag: release-2.7
- File paths:
  - user-guide/auto_sync/
- Relevant snippet: Documents automated sync behavior and control flags for production safety.

## Candidate B
- Kind: docs_example
- Title: Amazon EKS guide for Argo CD application creation
- URL: https://docs.aws.amazon.com/eks/latest/userguide/argocd-create-application.html
- Commit/Tag: latest
- File paths:
  - argocd-create-application.html
- Relevant snippet: Shows sync policy choices and cautions for automated sync in cluster operations.
