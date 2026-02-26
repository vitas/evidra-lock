# Evidra CLI --- API Client Specification

Status: Draft\
Scope: API-first client for CI and automation

The Evidra CLI is a thin API client for the Evidra REST service.

It:

-   Calls adapters (optional)
-   Constructs ToolInvocation
-   Sends request to Evidra API
-   Prints decision summary

It does NOT evaluate policy locally.

------------------------------------------------------------------------

# 1. Command Structure

    evidra <command> [subcommand] [flags]

Primary command:

    evidra validate

CI helper mode:

    evidra ci terraform --plan tfplan.bin --env production

------------------------------------------------------------------------

# 2. Behavior

1.  Extract parameters (via adapter or payload file)
2.  Build ToolInvocation
3.  POST /v1/validate
4.  Render decision

All decisions are made by the REST API.

------------------------------------------------------------------------

# 3. Output Modes

## Default (Human-readable)

✔ Allowed\
✖ Denied with reasons

## JSON Mode

    evidra validate --json

Outputs structured decision JSON.

------------------------------------------------------------------------

# 4. Exit Codes

  Code   Meaning
  ------ ------------------------------
  0      Allowed
  1      Denied (with --fail-on-deny)
  2      Client error
  3      API error

------------------------------------------------------------------------

# 5. Principle

> The CLI is an API client, not the engine.
