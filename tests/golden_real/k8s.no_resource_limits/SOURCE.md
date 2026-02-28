# Source Attribution

Rule: k8s.no_resource_limits

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: Kubernetes manage resources for containers
- URL: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
- Commit/Tag: latest
- File paths:
  - docs/concepts/configuration/manage-resources-containers
- Relevant snippet: Defines CPU/memory requests and limits and operational impact.

## Candidate B
- Kind: docs_example
- Title: Kubernetes LimitRange policy
- URL: https://kubernetes.io/docs/concepts/policy/limit-range/
- Commit/Tag: latest
- File paths:
  - docs/concepts/policy/limit-range
- Relevant snippet: Shows namespace policy to enforce default/min/max container limits.
