# Evidra Real-World Corpus (Phase 1)

This corpus is the source-of-truth for **real public references** used to derive `tests/golden_real` fixtures in Phase 2.

## Scope
- Input: all rule IDs discovered from:
  - `policy/bundles/ops-v0.1/evidra/policy/rules/*.rego` (`deny[...]` and `warn[...]` labels)
  - `policy/bundles/ops-v0.1/evidra/data/rule_hints/data.json` (hint-only labels such as `sys.unlabeled_deny`)
- Output: `sources.json` entries per rule ID, each marked:
  - `found_candidates`: at least two real candidates collected
  - `no_public_candidates`: no clean external analog found after search

## Selection Criteria
- Real public source only: official docs, public repos, public issue threads, public incident writeups.
- Concrete traceability fields required per candidate:
  - `url`
  - `evidence.file_paths`
  - `evidence.commit` (tag, release version, or `n/a` for non-repo incident pages)
  - `evidence.snippet_description`
- Preference order:
  1. Versioned vendor/official docs
  2. Public repo examples with stable paths
  3. Incident writeups for operational controls

## Non-goals in Phase 1
- No fixture JSON generation.
- No policy changes.
- No synthetic or invented payloads.

## Input Schema Discovery (for Phase 2 derivation)
Validated from code:
- Policy evaluator builds OPA input from invocations in `pkg/policy/policy.go`.
- Scenario-to-invocation mapping is built in `pkg/validate/validate.go`.
- Action payload shape used by rules:
  - `actions[].kind` (e.g., `terraform.plan`, `kubectl.apply`, `argocd.sync`)
  - `actions[].target`
  - `actions[].payload`
  - `actions[].risk_tags`
  - top-level `environment`, `actor`, `source`.
