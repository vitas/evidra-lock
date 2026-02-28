# Source Attribution

Rule: ops.unapproved_change

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: Amazon EKS Argo CD application workflow
- URL: https://docs.aws.amazon.com/eks/latest/userguide/argocd-create-application.html
- Commit/Tag: latest
- File paths:
  - argocd-create-application.html
- Relevant snippet: Shows explicit sync controls and operational approval points before deployment.

## Candidate B
- Kind: incident_writeup
- Title: GitLab database incident postmortem
- URL: https://about.gitlab.com/blog/2017/02/01/gitlab-dot-com-database-incident/
- Commit/Tag: n/a
- File paths:
  - (none)
- Relevant snippet: Incident emphasizes procedural controls and approvals for production-impacting changes.
