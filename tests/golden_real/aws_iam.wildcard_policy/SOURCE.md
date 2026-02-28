# Source Attribution

Rule: aws_iam.wildcard_policy

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: tfsec: no-policy-wildcards
- URL: https://aquasecurity.github.io/tfsec/v1.28.13/checks/aws/iam/no-policy-wildcards/
- Commit/Tag: v1.28.13
- File paths:
  - checks/aws/iam/no-policy-wildcards
- Relevant snippet: Contains insecure and secure Terraform IAM policy examples around wildcard Action/Resource.

## Candidate B
- Kind: docs_example
- Title: AWS IAM security best practices
- URL: https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html
- Commit/Tag: latest
- File paths:
  - best-practices.html
- Relevant snippet: Recommends least-privilege and avoiding wildcard permissions.
