
  ┌───────────────────────────────────────────────────────────────┬─────────────────────────────────────────────────────────────────────────────────────┐
  │                             File                              │                                       Change                                        │
  ├───────────────────────────────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────┤
  │ pkg/policy/policy.go                                          │ Removed Context["environment"] fallback (lines 91-93)                               │
  ├───────────────────────────────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────┤
  │ pkg/invocation/invocation.go                                  │ Removed "environment": true from allowedContextKeys                                 │
  ├───────────────────────────────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────┤
  │ pkg/runtime/policy_wiring_test.go                             │ Moved env from Context map to Environment struct field                              │
  ├───────────────────────────────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────┤
  │ 7 Rego test files                                             │ Removed "context": {"environment": "..."} lines                                     │
  ├───────────────────────────────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────┤
  │ 9 JSON fixture files                                          │ Moved environment to top-level field, removed context                               │
  ├───────────────────────────────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────┤
  │ policy/bundles/ops-v0.1/evidra/data/params/data.json          │ Added "production": 3 override for mass_delete threshold                            │
  ├───────────────────────────────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────┤
  │ policy/bundles/ops-v0.1/tests/env_specific_behavior_test.rego │ New — proves env-specific behavior (4 deletes denied in production, allowed in dev) │
  └───────────────────────────────────────────────────────────────┴─────────────────────────────────────────────────────────────────────────────────────┘

  Canonical rule

  ToolInvocation.Environment is the single source of truth for environment. Context["environment"] is now rejected by strict validation. OPA reads input.environment (top-level) via
  resolve_param/resolve_list_param helpers.

  Test results

  - OPA: 18/18 PASS (including 2 new env-specific proof tests)
  - Go: all 15 packages PASS