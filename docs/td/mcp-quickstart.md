# MCP Quickstart (Local)

This guide shows the fastest way to run Evidra locally and connect it as
an MCP server.

## What you get

-   An MCP server that returns deterministic decisions: **allow / deny /
    observe**
-   A persistent, hash-chained evidence log (audit trail)
-   A simple "gate" for AI agents: validate first, execute only if
    allowed

------------------------------------------------------------------------

## Step 0 --- Download binaries (recommended)

Download the latest prebuilt binaries from Releases:

-   `evidra` (CLI)
-   `evidra-mcp` (MCP server)

> If you prefer to build from source, see: `build-from-source.md`

------------------------------------------------------------------------

## Step 1 --- Verify installation

Run:

``` bash
evidra --help
evidra-mcp --help
```

If both commands print help, you are good.

------------------------------------------------------------------------

## Step 2 --- Start the MCP server

Run the server in a terminal:

``` bash
evidra-mcp
```

Keep it running.

> Evidence store default path: `~/.evidra/evidence`

------------------------------------------------------------------------

## Step 3 --- Connect an MCP client

Generic MCP config template:

``` json
{
  "mcpServers": {
    "evidra": {
      "command": "evidra-mcp",
      "args": []
    }
  }
}
```

Place this config in your MCP client configuration (Codex, Claude, Gemini, etc.).

------------------------------------------------------------------------

## Step 4 --- Test validation

CLI test:

``` bash
evidra validate <your-file>
```

Expected output shape:

``` json
{
  "decision": "deny",
  "violations": [
    {
      "rule_id": "example.rule",
      "severity": "high",
      "message": "Human-readable reason",
      "hint": "How to fix (optional)"
    }
  ],
  "evidence_id": "..."
}
```

------------------------------------------------------------------------

## Step 5 --- Enforce gating logic

Agent flow:

1.  Generate plan/manifests
2.  Call `validate`
3.  If `decision == allow` → apply
4.  If `decision == deny` → stop

------------------------------------------------------------------------

## Step 6 --- Inspect evidence

``` bash
evidra evidence inspect <evidence_id>
```
