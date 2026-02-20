# podman-basic

Minimal declarative Podman surface for image provenance actions in ops profile.

Supported operations:
- `images`
- `tag`
- `push`

Why this is intentionally limited:
- excludes `build`, `run`, `exec`, `login`, `pull`
- no free-form args
- explicit argv templates only

Policy guardrails:
- Registry allowlist from `policy/data.example.json` (`podman.allowed_registries`)
- Denied tags from `policy/data.example.json` (`podman.deny_tags`)
- `latest` is denied by default
- Push/tag decisions are environment-aware (`context.environment`)

Examples:

Allowed tag (dev):

```json
{
  "actor": {"type":"human","id":"ops-user","origin":"mcp"},
  "tool":"podman",
  "operation":"tag",
  "params":{
    "source":"myimg:1.2.3",
    "target":"registry.example.com/team/app:1.2.3"
  },
  "context":{"environment":"dev"}
}
```

Allowed push (dev):

```json
{
  "actor": {"type":"human","id":"ops-user","origin":"mcp"},
  "tool":"podman",
  "operation":"push",
  "params":{"image":"registry.example.com/team/app:1.2.3"},
  "context":{"environment":"dev"}
}
```

Denied push (`latest`):

```json
{
  "actor": {"type":"human","id":"ops-user","origin":"mcp"},
  "tool":"podman",
  "operation":"push",
  "params":{"image":"registry.example.com/team/app:latest"},
  "context":{"environment":"prod"}
}
```

Expected policy shape for allowed push in prod:
- `risk_level`: `critical`
- `reason`: `allowed_push_prod`

Denied push to unknown registry:

```json
{
  "actor": {"type":"human","id":"ops-user","origin":"mcp"},
  "tool":"podman",
  "operation":"push",
  "params":{"image":"docker.io/library/nginx:1.25.0"},
  "context":{"environment":"dev"}
}
```

