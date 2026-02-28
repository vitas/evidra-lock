# Source Attribution

Rule: ops.terraform_metadata_only

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
- Relevant snippet: Specifies rich change model fields needed for policy-grade evaluation.

## Candidate B
- Kind: github_repo
- Title: Terraform issue: missing details in expected JSON structures
- URL: https://github.com/hashicorp/terraform/issues/35674
- Commit/Tag: n/a
- File paths:
  - issues/35674
- Relevant snippet: Demonstrates real-world scenarios where expected semantic fields are absent.
