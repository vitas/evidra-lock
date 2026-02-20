# Policy Templates v0.1

Policy templates provide small, deterministic Rego starting points for different usage modes.

## Purpose
- Keep policy behavior explicit and reviewable.
- Reuse standardized reason codes across templates.
- Avoid ad hoc policy drift.

## Standardized Reason Codes
- Templates use controlled reason code values from `policy/reason_codes.rego`.
- This keeps deny/allow reasons machine-checkable and audit-friendly.

## Risk Levels
- `low`: routine
- `medium`: change
- `high`: potentially destructive
- `critical`: default/blocked/unknown

## Template Selection
- `dev_safe.rego`: local development with a small safe allow set.
- `regulated_dev.rego`: stricter developer flow with write-like operations denied.
- `ci_agent.rego`: CI-focused policy with explicit high-risk denials.

## Running OPA Tests
- Run template tests with:
  - `opa test ./policy/...`
