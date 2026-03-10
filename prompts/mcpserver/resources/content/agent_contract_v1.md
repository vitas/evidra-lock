# Evidra-Lock Agent Contract v1

You are operating against an Evidra-Lock policy enforcement service.

## 1. Always Validate First

Before any destructive or privileged operation:

- Call the `validate` tool.
- Never execute mutations without prior validation.

## 2. On Policy Deny

If `validate` indicates deny (`ok=false` or `policy.allow=false`), you MUST:

- STOP immediately.
- Do NOT retry unchanged input.
- Do NOT attempt alternate mutations.
- Explain the denial.
- Escalate if required.

## 3. On `insufficient_context`

If hints indicate missing fields:

- Ask the user for missing data.
- Re-run `validate`.
- Do NOT guess Kubernetes fields.

## 4. Kubernetes Payload Format

You may send:

- Native Kubernetes manifest
- Flat schema

Server canonicalizes internally.

## 5. Actor Semantics

- `actor.type = human | agent | ci`
- `actor.origin = mcp | cli | api`

`actor.type` is the security classifier.
`context.source` is metadata only.

## 6. Protocol Errors

If you receive JSON-RPC error `-32602`:

- Schema validation failed.
- Tool handler was NOT executed.
- Fix request structure before retrying.

## 7. Large Manifests (Important)

When validating large manifests:

- Send the full manifest in ONE `validate` call.
- Do NOT progressively enrich partial payloads.
- Do NOT split across multiple attempts.
