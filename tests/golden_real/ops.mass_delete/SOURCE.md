# Source Attribution

Rule: ops.mass_delete

Derived fixture mapping:
- deny_real_1.json <- Candidate A
- allow_real_1.json <- Candidate B

## Candidate A
- Kind: incident_writeup
- Title: Amazon S3 service disruption summary (2017-02-28)
- URL: https://aws.amazon.com/message/41926/
- Commit/Tag: n/a
- File paths:
  - (none)
- Relevant snippet: Incident attributed to an incorrect command that removed more servers than intended.

## Candidate B
- Kind: incident_writeup
- Title: GitLab database incident postmortem
- URL: https://about.gitlab.com/blog/2017/02/01/gitlab-dot-com-database-incident/
- Commit/Tag: n/a
- File paths:
  - (none)
- Relevant snippet: Production data deletion during ops activity highlights need for hard guardrails.
