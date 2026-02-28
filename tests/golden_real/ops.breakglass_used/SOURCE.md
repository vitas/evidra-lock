# Source Attribution

Rule: ops.breakglass_used

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: docs_example
- Title: Google Cloud Binary Authorization breakglass
- URL: https://cloud.google.com/binary-authorization/docs/using-breakglass
- Commit/Tag: latest
- File paths:
  - binary-authorization/docs/using-breakglass
- Relevant snippet: Documents explicit emergency bypass workflow with audit logging.

## Candidate B
- Kind: docs_example
- Title: AWS Well-Architected emergency process for permissions
- URL: https://docs.aws.amazon.com/wellarchitected/latest/framework/sec_permissions_emergency_process.html
- Commit/Tag: latest
- File paths:
  - framework/sec_permissions_emergency_process.html
- Relevant snippet: Describes emergency access controls and post-incident review requirements.
