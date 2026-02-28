# Source Attribution

Rule: terraform.sg_open_world

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: tfsec: no-public-ingress-sgr
- URL: https://aquasecurity.github.io/tfsec/v1.6.2/checks/aws/ec2/no-public-ingress-sgr/
- Commit/Tag: v1.6.2
- File paths:
  - checks/aws/ec2/no-public-ingress-sgr
- Relevant snippet: Shows insecure world-open ingress and secure restricted CIDR alternatives.

## Candidate B
- Kind: docs_example
- Title: AWS Control Tower EC2 high-risk port control
- URL: https://docs.aws.amazon.com/controltower/latest/controlreference/ec2-rules.html
- Commit/Tag: latest
- File paths:
  - controlreference/ec2-rules.html
- Relevant snippet: Control includes detection of 0.0.0.0/0 on high-risk ports including 22 and 3389.
