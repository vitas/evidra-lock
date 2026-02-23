# FAQ

## Why not generic shell execution?
Because unrestricted shell commands break deterministic control boundaries and make policy/evidence guardrails weak.

## Why OPA?
OPA provides deterministic policy-as-code with explicit allow/risk/reason outputs.

## Why segmented evidence?
Segmented logs scale better operationally and allow stable sealed units for export/forward workflows.

## How does prod vs dev work?
Callers set `context.environment` in ToolInvocation. Policies use this field to differentiate risk/allow decisions.

## Can I write my own policy?
Yes. Replace or extend `policy/profiles/ops-v0.1` with your own Rego modules and data; reload the MCP server or offline CLI with `--policy`/`--data` to test changes.

## Is this SaaS?
No. v0.1 is local-first and does not require remote services.

## What is forwarder cursor for?
It is a local bookmark (`forwarder_state.json`) for tracking acknowledged evidence position for future forwarding/export pipelines.
