# Source Attribution

Rule: ops.public_exposure

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: AWS S3 Block Public Access
- URL: https://docs.aws.amazon.com/AmazonS3/latest/userguide/access-control-block-public-access.html
- Commit/Tag: latest
- File paths:
  - access-control-block-public-access.html
- Relevant snippet: Canonical guidance for preventing unintended public exposure in S3.

## Candidate B
- Kind: incident_writeup
- Title: Verizon customer records leak linked to public S3 bucket
- URL: https://www.forbes.com/sites/leemathews/2017/07/13/millions-of-verizon-customers-exposed-by-third-party-leak/
- Commit/Tag: n/a
- File paths:
  - (none)
- Relevant snippet: Public incident coverage of data exposure caused by public cloud storage configuration.
