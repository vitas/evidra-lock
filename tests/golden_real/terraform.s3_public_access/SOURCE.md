# Source Attribution

Rule: terraform.s3_public_access

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: tfsec: block-public-policy
- URL: https://aquasecurity.github.io/tfsec/v1.28.13/checks/aws/s3/block-public-policy/
- Commit/Tag: v1.28.13
- File paths:
  - checks/aws/s3/block-public-policy
- Relevant snippet: Terraform examples for enabling/disabling block public policy settings.

## Candidate B
- Kind: docs_example
- Title: AWS S3 Block Public Access
- URL: https://docs.aws.amazon.com/AmazonS3/latest/userguide/access-control-block-public-access.html
- Commit/Tag: latest
- File paths:
  - access-control-block-public-access.html
- Relevant snippet: Defines all four block-public-access controls and their effects.
