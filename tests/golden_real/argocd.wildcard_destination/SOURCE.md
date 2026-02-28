# Source Attribution

Rule: argocd.wildcard_destination

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: Argo CD AppProject specification
- URL: https://argo-cd.readthedocs.io/en/latest/operator-manual/project-specification/
- Commit/Tag: latest
- File paths:
  - operator-manual/project-specification/
- Relevant snippet: Shows destinations schema where wildcard namespace/server can be configured.

## Candidate B
- Kind: docs_example
- Title: Amazon EKS Argo CD project guidance
- URL: https://docs.aws.amazon.com/eks/latest/userguide/argocd-projects.html
- Commit/Tag: latest
- File paths:
  - argocd-projects.html
- Relevant snippet: Shows destination restrictions and project scoping for Argo CD.
