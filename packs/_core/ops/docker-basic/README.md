# docker-basic

Minimal declarative Docker surface for image provenance operations in ops profile.

Supported operations:
- `images`
- `tag`
- `push`

Guardrails:
- Registry allowlist (`policy/data.example.json` -> `docker.allowed_registries`)
- Tag discipline (`docker.deny_tags`, defaults deny `latest`)
- Missing tag is treated as `latest` and denied
- No free-form args, no generic wrapper

Examples:

Allowed tag:

```json
{
  "actor": {"type":"human","id":"ops-user","origin":"mcp"},
  "tool":"docker",
  "operation":"tag",
  "params":{
    "source":"app:1.2.3",
    "target":"registry.example.com/team/app:1.2.3"
  },
  "context":{"environment":"dev"}
}
```

Allowed push in prod:

```json
{
  "actor": {"type":"human","id":"ops-user","origin":"mcp"},
  "tool":"docker",
  "operation":"push",
  "params":{"image":"registry.example.com/team/app:1.2.3"},
  "context":{"environment":"prod"}
}
```

Denied push (`latest`):

```json
{
  "actor": {"type":"human","id":"ops-user","origin":"mcp"},
  "tool":"docker",
  "operation":"push",
  "params":{"image":"registry.example.com/team/app:latest"},
  "context":{"environment":"dev"}
}
```

