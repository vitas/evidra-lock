# Source Attribution

Rule: aws_s3.no_encryption

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: tfsec: enable-bucket-encryption
- URL: https://aquasecurity.github.io/tfsec/v1.28.11/checks/aws/s3/enable-bucket-encryption/
- Commit/Tag: v1.28.11
- File paths:
  - checks/aws/s3/enable-bucket-encryption
- Relevant snippet: Shows insecure/secure Terraform S3 encryption configurations.

## Candidate B
- Kind: docs_example
- Title: AWS S3 default bucket encryption
- URL: https://docs.aws.amazon.com/AmazonS3/latest/userguide/default-bucket-encryption.html
- Commit/Tag: latest
- File paths:
  - default-bucket-encryption.html
- Relevant snippet: Canonical AWS guidance for enforcing default bucket encryption.
