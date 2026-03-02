# Model Behavior and Determinism

## Architectural Boundary

Evidra-mcp exposes two tools:

- `validate`
- `get_event`

It does not execute or intercept infrastructure mutations (e.g., `kubectl apply`, `terraform apply`, `argocd sync`).
Mutation execution occurs outside Evidra.

Because of this:

- Evidra cannot enforce "validate-required" before execution.
- Evidra cannot block execution after a deny.
- Enforcement is limited to behavior inside the `validate` tool.

## Tool-Use Variance Across Models

LLM behavior varies by model capability:

- Stronger models (e.g., Sonnet) consistently invoke `validate`.
- Smaller models (e.g., Haiku) may occasionally skip tool invocation.
- Smaller models (e.g., Haiku) may produce text-only refusals.
- Smaller models (e.g., Haiku) may retry `validate` with unchanged parameters.

This variance is expected and does not indicate an engine defect.

Engine correctness is independent of model compliance variance.

## Deterministic Engine Guarantees

Evidra guarantees determinism at the policy layer:

- Canonicalization produces stable normalized input.
- Policy evaluation is deterministic.
- Identical inputs produce identical decisions.
- Deny-loop prevention ensures repeated identical denied intents are not re-evaluated.

Determinism applies to policy evaluation, not to agent reasoning.

## CI Model Policy

For continuous integration:

- Sonnet is used as the blocking model.
- Haiku is used as a non-blocking signal run to detect behavioral variance.

This ensures production stability while monitoring cross-model behavior.

## Error Classification

Behavior gates (e.g., `stop_after_deny`) are tool-level errors.
Transport-level schema failures (e.g., JSON-RPC `-32602`) occur before the handler runs.

See [PROTOCOL_ERRORS.md](PROTOCOL_ERRORS.md) for the full error taxonomy.
