# Policy overview

## Layout

- Policy lives in `policy/profiles/ops-v0.1`.
- `policy.rego` at the profile root loads `policy/decision.rego`.
- Rule files live under `policy/decision.rego` (aggregator) and `policy/rules/deny_*.rego` / `policy/rules/warn_*.rego`.
- Thresholds, hints, and override data sit in `data.json`.

## Rule structure

- Deny rules declare `deny[label] = message` in their file; warn rules use `warn[label]`.
- Keep each rule small (10–40 lines) and use `policy/rules/00_normalize.rego` helpers for tags, namespaces, and payload extraction.
- Rule labels feed into the decision aggregator’s `hits` and, via `data.json`, into human-friendly hints.

## Testing

- Run `opa test policy/profiles/ops-v0.1` to execute the bundled rule suite.
- Keep tests under `policy/tests/` aligned with the rule names; they already assert `decision.allow`, `reason`, `hints`, etc.

## Adding a rule (example)

1. Create a file `policy/rules/deny_example.rego` with:

```rego
package evidra.policy.rules

deny["example-block"] = "example rule fired" if {
  ...
}
```

2. Add hint text under `data.json` (optional).
3. Add a test under `policy/tests/deny_example_test.rego` that exercises the new rule.
