# docker-compose-basic

Minimal declarative Docker Compose surface for controlled ops actions.

Supported operations:
- `config`
- `ps`
- `up-service`
- `restart-service`

Design constraints:
- No `down` in v0.1
- No multiple `-f` files
- No `--env-file`
- No free-form args

Policy behavior:
- `config` and `ps` are low-risk read operations
- `up-service` and `restart-service` are write operations:
  - dev: high risk
  - prod: critical risk
- `file` must be under allowed prefixes from `policy/data.example.json`

Example (`config`):

```json
{
  "actor": {"type":"human","id":"ops-user","origin":"mcp"},
  "tool":"docker-compose",
  "operation":"config",
  "params":{"file":"./deploy/compose.yaml"},
  "context":{"environment":"dev"}
}
```

Example (`up-service` in prod):

```json
{
  "actor": {"type":"human","id":"ops-user","origin":"mcp"},
  "tool":"docker-compose",
  "operation":"up-service",
  "params":{"file":"./deploy/compose.yaml","service":"api"},
  "context":{"environment":"prod"}
}
```

