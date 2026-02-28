# Source Attribution

Rule: aws_iam.wildcard_principal

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: AWS Principal element reference
- URL: https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_principal.html
- Commit/Tag: latest
- File paths:
  - reference_policies_elements_principal.html
- Relevant snippet: Defines Principal behavior in trust policies and why broad principals are dangerous.

## Candidate B
- Kind: docs_example
- Title: IAM Access Analyzer policy checks
- URL: https://docs.aws.amazon.com/IAM/latest/UserGuide/access-analyzer-reference-policy-checks.html
- Commit/Tag: latest
- File paths:
  - access-analyzer-reference-policy-checks.html
- Relevant snippet: Enumerates problematic trust-policy patterns and recommended narrowing.
