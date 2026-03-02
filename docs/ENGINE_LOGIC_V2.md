# Evidra Engine Logic v2

Canonical reference for current validation engine behavior.

## Scope

This document covers only engine logic:
- Go input construction
- policy canonicalization boundary
- rule evaluation contract
- decision aggregation contract

It does not describe API/UI/evidence-store internals.

## Pipeline

1. Go builds OPA input with `input.actions`.
2. `canonicalize.rego` projects action payloads into normalized flat shape.
3. Rules and decision aggregation read `defaults.actions` only.
4. Decision is returned with `allow`, `risk_level`, `reason`, `reasons`, `hits`, and `hints`.

## Input Formation (Go)

Input is assembled in [`pkg/policy/policy.go`](../pkg/policy/policy.go):
- `buildActionList(params, origin)` extracts actions from `params.action` (and `params.actions` for non-MCP origins).
- `Engine.Evaluate` passes `actor`, `tool`, `operation`, `context`, `environment`, and `actions` into OPA input.

For scenario execution, per-action invocation is created in [`pkg/validate/validate.go`](../pkg/validate/validate.go), then evaluated through the same engine path.

MCP validate tool input schema source of truth:
- JSON file: [`pkg/mcpserver/schemas/validate.schema.json`](../pkg/mcpserver/schemas/validate.schema.json)
- Embedded loader: [`pkg/mcpserver/schema_embed.go`](../pkg/mcpserver/schema_embed.go)

## Canonicalization Boundary

Canonicalization is defined in [`policy/bundles/ops-v0.1/evidra/policy/canonicalize.rego`](../policy/bundles/ops-v0.1/evidra/policy/canonicalize.rego):
- package: `evidra.policy.defaults`
- export: `actions := [canonicalize_action(action) | action := input.actions[_]]`
- K8s-native payloads for `kubectl.apply`/`oc.apply` are normalized.
- Non-K8s actions pass through unchanged.

Normalized payload fields consumed by current rules:
- `namespace`
- `resource`
- `containers`
- `init_containers`
- `volumes`
- `host_pid`
- `host_ipc`
- `host_network`
- container `security_context` (camelCase `securityContext` normalized here)

## Rule Contract

Rules must read normalized actions only:
- `action := defaults.actions[_]`
- no direct `input.actions` in rules or decision aggregation

Flat helpers live in [`policy/bundles/ops-v0.1/evidra/policy/defaults.rego`](../policy/bundles/ops-v0.1/evidra/policy/defaults.rego) (for example, namespace/tag/container helpers).

`ops.insufficient_context` behavior is implemented in
[`policy/bundles/ops-v0.1/evidra/policy/rules/deny_insufficient_context.rego`](../policy/bundles/ops-v0.1/evidra/policy/rules/deny_insufficient_context.rego):
- deny semantics stay fail-closed for destructive operations without sufficient context
- reason/hint UX distinguishes:
  - missing required data
  - unsupported payload shape (wrong types/structure)
- decision hints include per-operation skeletons and shape guidance

## Decision Aggregation

Decision aggregation is in [`policy/bundles/ops-v0.1/evidra/policy/decision.rego`](../policy/bundles/ops-v0.1/evidra/policy/decision.rego):
- collects `deny` + `warn` labels/messages
- computes `allow`
- computes `risk_level`
- deduplicates `reasons`, `hits`, and `hints`
- applies actor-aware golden gating (Layer 2) using bundle data:
  - [`policy/bundles/ops-v0.1/evidra/policy/data.json`](../policy/bundles/ops-v0.1/evidra/policy/data.json)
  - `golden.rule_ids`
  - `agent_kill_switch.enabled`

Returned shape:
- `allow`
- `risk_level`
- `reason`
- `reasons`
- `hits`
- `hints`
- `actor_kind` (additive)
- `golden_hits` (additive)
- `blocked_by_agent_kill_switch` (additive)

### Deny Hints

Hint aggregation is in
[`policy/bundles/ops-v0.1/evidra/policy/decision.rego`](../policy/bundles/ops-v0.1/evidra/policy/decision.rego),
and remains backward-compatible as `hints: []string`.

Minimum hint categories:
- Agent kill switch block: when `blocked_by_agent_kill_switch` is true, hints include
  a kill-switch message with `golden_hits` and an explicit stop action.
- Insufficient context (missing data): when `ops.insufficient_context` denies without
  unsupported-shape signals, hints include missing-data guidance and next-request fields.
- Unsupported Kubernetes shape: when insufficient-context deny is tied to unsupported
  Kubernetes manifest shape, hints include template pod-spec-layout vs flat-schema guidance.

### Actor Field Semantics

- `input.actor.type` (`human|agent|ci`): **YES**, security classification and gating input.
- `input.actor.origin` (`mcp|cli|api`): **NO**, transport metadata only.
- `input.context.source` (free string): **NO**, metadata only; never used for security classification.

Actor classification for Layer 2:
- `human` when `input.actor.type == "human"`
- `agent` when `input.actor.type == "agent"`
- `ci` when `input.actor.type == "ci"`
- unknown/missing actor type defaults to `agent` (safety-first)

CI behavior:
- CI is treated like `agent` for kill-switch gating.

## Invariants

1. `input.actions` is only read in `canonicalize.rego`.
2. K8s shape/casing knowledge stays in `canonicalize.rego`.
3. Rules and decision logic are format-agnostic and operate on flat normalized payload.

These invariants are enforced by policy boundary guard tests in
[`pkg/policy/policy_input_actions_guard_test.go`](../pkg/policy/policy_input_actions_guard_test.go).

## Engine v2 Regression Set

The following checks must stay green for engine-v2 stability:

- `opa test policy/bundles/ops-v0.1/ -v` (canonicalizer + rule/decision behavior)
- `go test ./...` (engine integration + guard tests)
- `go test ./pkg/policy -run TestPolicyBoundary` (explicit boundary guard:
  `input.actions` and K8s shape/casing are forbidden outside `canonicalize.rego`)

CI runs the first two checks in
[`.github/workflows/ci.yml`](../.github/workflows/ci.yml).
