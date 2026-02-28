# Source Attribution

Rule: aws_s3.no_versioning_prod

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: tfsec: enable-versioning
- URL: https://aquasecurity.github.io/tfsec/v1.28.6/checks/aws/s3/enable-versioning/
- Commit/Tag: v1.28.6
- File paths:
  - checks/aws/s3/enable-versioning
- Relevant snippet: Provides insecure and secure Terraform versioning examples for S3.

## Candidate B
- Kind: docs_example
- Title: AWS S3 versioning user guide
- URL: https://docs.aws.amazon.com/AmazonS3/latest/userguide/Versioning.html
- Commit/Tag: latest
- File paths:
  - Versioning.html
- Relevant snippet: Describes versioning as protection against accidental deletes/overwrites.
