## Quick start (MCP in 2 minutes)

1)  Download `evidra` + `evidra-mcp` from Releases\
2)  Start server:

``` bash
evidra-mcp
```

3)  Add MCP config:

``` json
{ "mcpServers": { "evidra": { "command": "evidra-mcp", "args": [] } } }
```

4)  Agent flow:

-   call `validate`
-   if `allow` → apply
-   if `deny` → stop

Full guide: `docs/mcp-quickstart.md`\
Build from source: `docs/build-from-source.md`
