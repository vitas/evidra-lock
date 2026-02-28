# Source Attribution

Rule: ops.insufficient_context

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: Terraform JSON output format
- URL: https://developer.hashicorp.com/terraform/internals/json-format
- Commit/Tag: latest
- File paths:
  - terraform/internals/json-format
- Relevant snippet: Defines semantic fields required to reason about plan safety beyond counts.

## Candidate B
- Kind: github_repo
- Title: Terraform issue: incomplete plan JSON extraction edge cases
- URL: https://github.com/hashicorp/terraform/issues/35674
- Commit/Tag: n/a
- File paths:
  - issues/35674
- Relevant snippet: Public issue showing practical gaps when expected plan details are absent.
