# FAQ

## Why not generic shell execution?
Because unrestricted shell commands break deterministic control boundaries and make policy/evidence guardrails weak.

## Why OPA?
OPA provides deterministic policy-as-code with explicit allow/risk/reason outputs.

## Why segmented evidence?
Segmented logs scale better operationally and allow stable sealed units for export/forward workflows.

## How does prod vs dev work?
Pass `--environment` (or set `EVIDRA_ENVIRONMENT`) to provide an opaque environment label. Policy rules use `resolve_param` / `resolve_list_param` helpers to look up environment-specific thresholds and lists from `data.json`.

## Can I write my own policy?
Yes. Replace or extend `policy/bundles/ops-v0.1` with your own Rego modules and data; reload the MCP server or offline CLI with `--bundle` to test changes.

## Is this SaaS?
No. v0.1 is local-first and does not require remote services.

## What is forwarder cursor for?
It is a local bookmark (`forwarder_state.json`) for tracking acknowledged evidence position for future forwarding/export pipelines.
