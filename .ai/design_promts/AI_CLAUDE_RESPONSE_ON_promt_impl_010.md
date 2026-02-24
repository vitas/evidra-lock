 1. Why do we need environment?

  It powers data-driven param resolution. Three rules depend on it today through resolve_param / resolve_list_param:

  ┌─────────────────────────────────┬─────────────────────────────┬───────────────────────────────────────────────────────────────────────┐
  │              Rule               │          Param key          │                     What changes per environment                      │
  ├─────────────────────────────────┼─────────────────────────────┼───────────────────────────────────────────────────────────────────────┤
  │ deny_mass_delete.rego           │ ops.mass_delete.max_deletes │ Threshold (e.g., 5 in prod, 10 in staging)                            │
  ├─────────────────────────────────┼─────────────────────────────┼───────────────────────────────────────────────────────────────────────┤
  │ deny_prod_without_approval.rego │ k8s.namespaces.protected    │ Which namespaces need approval (e.g., ["prod"] default, could differ) │
  ├─────────────────────────────────┼─────────────────────────────┼───────────────────────────────────────────────────────────────────────┤
  │ deny_kube_system.rego           │ k8s.namespaces.restricted   │ Which namespaces are restricted (e.g., ["kube-system"] default)       │
  └─────────────────────────────────┴─────────────────────────────┴───────────────────────────────────────────────────────────────────────┘

  What breaks if removed: resolve_param would always hit the by_env["default"] clause. No crash, but you lose the ability to configure different thresholds/lists per environment — which is the entire point of
  the by_env data model.

  2. Why top-level instead of under context?

  Both paths exist. In pkg/policy/policy.go:89-93:

  if env := inv.Environment; env != "" {
      input["environment"] = env
  } else if ctx, ok := inv.Context["environment"]; ok {
      input["environment"] = ctx
  }

  The top-level Environment field on ToolInvocation was added during the bundle migration. The Context["environment"] path was the original approach (visible in existing example invocations like
  allowed_helm_upgrade_prod.json). The OPA input schema places it at input.environment (top-level) because resolve_param reads object.get(input, "environment", "") — not input.context.environment.

  This is a redundancy. There are two ways to provide the same value. The architecture docs (.ai/AI_CLAUDE_SYSTEM_DESIGN_OPA_BUNDLES.md section 5) specify input.environment as a top-level field in the OPA
  input, which is correct. The dual entry path (ToolInvocation.Environment vs Context["environment"]) is where the ambiguity lives.

  3. Required or optional?

  Optional. When absent:
  - object.get(input, "environment", "") returns ""
  - The second resolve_param clause fires: env == "" → uses by_env["default"]
  - No rule crashes, no undefined errors

  No policy assumes environment always exists. The resolve_param helper has explicit clauses for empty string, missing env key, and env key present but not in by_env.

  Tests confirm this: test_resolve_param_default_env passes {"environment": ""} and gets by_env["default"].

  4. Only for param resolution, or direct branching?

  Only param resolution. Zero Rego rule files reference input.environment directly. The only file that reads it is defaults.rego inside resolve_param. Confirmed:

  grep 'input.environment' *.rego → only defaults.rego:24 (comment line)

  No env string literals ("prod", "staging", "production") exist in any rule .rego file. They appear only in test fixtures and in data.json (the string "prod" is a data value in k8s.namespaces.protected, not a
   Rego literal).

  5. True policy dimension or future-proofing?

  Currently half-realized. The infrastructure is real and tested:
  - resolve_param with 3-clause env resolution: works, tested
  - by_env data model: in place, but every param currently only has "default" — no actual env-specific overrides exist in the shipped data

  So today, environment is a real architectural dimension with a complete implementation but no shipped env-specific data. It's ready to use — someone just needs to add "staging": 10 next to "default": 5 in
  params/data.json to activate it.

  6. Tests that prove environment-specific behavior?

  Yes, at the Rego level:

  ┌──────────────────────────────────────────────────────┬────────────────────────────┬─────────────────────────────────────────────────────────────────┐
  │                         Test                         │            File            │                         What it proves                          │
  ├──────────────────────────────────────────────────────┼────────────────────────────┼─────────────────────────────────────────────────────────────────┤
  │ test_resolve_param_specific_env                      │ resolve_param_test.rego:14 │ staging → gets 10 instead of default 5                          │
  ├──────────────────────────────────────────────────────┼────────────────────────────┼─────────────────────────────────────────────────────────────────┤
  │ test_resolve_param_missing_env_falls_back_to_default │ resolve_param_test.rego:22 │ Unknown env → falls back to "default"                           │
  ├──────────────────────────────────────────────────────┼────────────────────────────┼─────────────────────────────────────────────────────────────────┤
  │ test_resolve_list_param_env_override                 │ resolve_param_test.rego:38 │ staging → gets ["kube-system", "monitoring"] instead of default │
  └──────────────────────────────────────────────────────┴────────────────────────────┴─────────────────────────────────────────────────────────────────┘

  At the Go level: pkg/runtime/policy_wiring_test.go:89-107 threads environment through Context["environment"] but doesn't test env-specific param differences (the test uses "prod" and "dev" but the shipped
  data.json has no env-specific entries, so the behavior is identical).

  ---
  Recommendation

  Keep environment — but fix the dual-entry-point ambiguity.

  The field is architecturally sound:
  - Real resolve_param infrastructure that's tested and working
  - Clean data-driven design (no hardcoded env literals in rules)
  - Correct optionality (graceful fallback to "default")

  What should be cleaned up:
  1. Pick one entry point. Either ToolInvocation.Environment (top-level) or Context["environment"] — not both. The Go code in policy.go:89-93 has a priority chain that's unnecessary complexity. Since the OPA
  input schema is input.environment (top-level), ToolInvocation.Environment is the natural match.
  2. Remove "environment" from allowedContextKeys in invocation.go:23 once you pick the top-level field.
  3. Add one real env-specific override to data.json to prove the system works end-to-end (e.g., "staging": 10 for ops.mass_delete.max_deletes).

  Files that depend on environment

  ┌───────┬─────────────────────────────────────────┬─────────────────────────────────────────────────────┐
  │ Layer │                  File                   │                     Dependency                      │
  ├───────┼─────────────────────────────────────────┼─────────────────────────────────────────────────────┤
  │ Rego  │ defaults.rego:25-45                     │ resolve_param reads input.environment               │
  ├───────┼─────────────────────────────────────────┼─────────────────────────────────────────────────────┤
  │ Rego  │ deny_mass_delete.rego:8                 │ calls resolve_param                                 │
  ├───────┼─────────────────────────────────────────┼─────────────────────────────────────────────────────┤
  │ Rego  │ deny_prod_without_approval.rego:8       │ calls resolve_list_param                            │
  ├───────┼─────────────────────────────────────────┼─────────────────────────────────────────────────────┤
  │ Rego  │ deny_kube_system.rego:8                 │ calls resolve_list_param                            │
  ├───────┼─────────────────────────────────────────┼─────────────────────────────────────────────────────┤
  │ Rego  │ resolve_param_test.rego                 │ 5 tests covering env resolution                     │
  ├───────┼─────────────────────────────────────────┼─────────────────────────────────────────────────────┤
  │ Go    │ pkg/policy/policy.go:89-93              │ sets input["environment"] from invocation           │
  ├───────┼─────────────────────────────────────────┼─────────────────────────────────────────────────────┤
  │ Go    │ pkg/validate/validate.go:95,319-324,359 │ resolves env from flag/envvar, stamps on invocation │
  ├───────┼─────────────────────────────────────────┼─────────────────────────────────────────────────────┤
  │ Go    │ pkg/evidence/types.go:35                │ EnvironmentLabel stored in evidence                 │
  ├───────┼─────────────────────────────────────────┼─────────────────────────────────────────────────────┤
  │ Go    │ cmd/evidra/main.go:67                   │ --environment flag                                  │
  ├───────┼─────────────────────────────────────────┼─────────────────────────────────────────────────────┤
  │ Go    │ cmd/evidra-mcp/main.go:41               │ --environment flag                                  │
  ├───────┼─────────────────────────────────────────┼─────────────────────────────────────────────────────┤
  │ Data  │ evidra/data/params/data.json            │ by_env structure (currently only "default")         │
  └───────┴─────────────────────────────────────────┴─────────────────────────────────────────────────────┘

Response form me:

You are a senior Go + Rego engineer. Perform a SMALL, CONTROLLED REFACTOR to eliminate the dual entry path for `environment` while preserving architecture invariants.

Goal:
Make `ToolInvocation.Environment` the single source of truth for OPA `input.environment`.
Remove legacy support for Context["environment"] as an input source.

Scope:
- Go runtime input construction
- Invocation/context validation
- CLI flag wiring (must remain)
- Tests and fixtures that rely on Context["environment"]
- Documentation/examples if they mention Context["environment"] (only minimal edits needed)

Hard constraints:
- `input.environment` remains OPTIONAL.
- Environment remains an opaque string label (no enums, no validation beyond "string/non-empty").
- No env literals in Rego rules.
- No change to policy semantics except removal of redundant input path.
- Preserve deterministic output ordering and canonical JSON behavior.
- No redesign, no new features.

Tasks (execute in order):

1) Canonicalize environment source in Go
- In pkg/policy/policy.go (or wherever OPA input is built):
  - Remove the fallback that reads env from inv.Context["environment"].
  - Only set input["environment"] when inv.Environment != "".
  - Ensure no other code path sets input["environment"].

2) Remove environment from context allowlist
- In pkg/invocation (allowedContextKeys or equivalent):
  - Remove "environment" from allowed context keys.
  - If there is a generic context passthrough, ensure it does not smuggle environment.

3) Ensure CLI still sets ToolInvocation.Environment
- Keep `--environment` flag (evidra and evidra-mcp).
- Confirm validator stamps the parsed value onto inv.Environment (not inv.Context).
- Remove any env-from-context normalization in validation.

4) Update tests and fixtures
- Find any tests/fixtures using Context["environment"].
- Update them to set ToolInvocation.Environment instead.
- Ensure all tests still pass:
  - go test ./...
  - opa test policies/bundles/ops-v0.1/... (or your bundle test path)

5) Add one end-to-end proof test for env-specific behavior
- Update evidra/data/params/data.json to include ONE real override:
  - ops.mass_delete.max_deletes has by_env.default and by_env.prod with different values.
- Add/extend an OPA test that proves different outcomes:
  - same action input, env="prod" -> different decision than env="" or env="staging" (as appropriate)
- No env literals inside rule logic; only in test input and data.json.

6) Repo sweep
- Ensure there are no remaining references to Context["environment"] in:
  - Go code
  - JSON fixtures
  - docs/examples
- Ensure determinism tests are unaffected.

Deliverables:
- List of modified files
- Short note describing the final canonical rule:
  "OPA input.environment is sourced only from ToolInvocation.Environment; context no longer carries environment."
- Confirmation of test results (go test + opa test)

Do not introduce any additional refactors outside this scope.