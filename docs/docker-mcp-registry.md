# Docker MCP Registry

Evidra is published to the [Docker MCP Registry](https://github.com/docker/mcp-registry), making it available as a one-click MCP server in Docker Desktop.

## Image

```
mcp/evidra
```

Docker builds and hosts the image from the project Dockerfile. The image is signed, includes provenance attestation and SBOM.

## Zero-config

The Docker image works out of the box with no configuration:

- **Embedded OPA bundle** — the `ops-v0.1` policy bundle is compiled into the binary via `go:embed` and extracted to `~/.evidra/bundles/ops-v0.1/` on first run.
- **Stdio transport** — the ENTRYPOINT runs `evidra-mcp` directly, which speaks MCP over stdin/stdout.
- **Deny cache enabled** — `EVIDRA_DENY_CACHE=true` is set by default in the registry config to prevent agent deny-loops.
- **No secrets required** — runs in offline mode (local policy evaluation, no API calls).

## Docker Desktop usage

After installing from the Docker MCP catalog, Evidra appears as an MCP server in Docker Desktop. AI agents connected through Docker Desktop MCP can call `validate` before destructive infrastructure operations.

## Registry files

The registry entry lives in the [docker/mcp-registry](https://github.com/docker/mcp-registry) repo under `servers/evidra/`:

| File | Purpose |
|---|---|
| `server.yaml` | Image name, category, tags, description, source repo, env vars |
| `tools.json` | Tool definitions: `validate` (6 args) and `get_event` (1 arg) |
| `readme.md` | Short description shown in the catalog |

### server.yaml

```yaml
name: evidra
image: mcp/evidra
type: server
meta:
  category: devops
  tags: [kubernetes, terraform, security, policy, opa, ai-agents, infrastructure]
about:
  title: Evidra
  description: Fail-closed policy guardrails for AI agents running kubectl, terraform, helm, argocd, and oc.
source:
  project: https://github.com/vitas/evidra
run:
  env:
    - name: EVIDRA_DENY_CACHE
      value: "true"
```

### Tools

**`validate`** — Evaluate OPA policy against a proposed infrastructure operation. Call before `kubectl apply`, `terraform apply`, `helm install`, `argocd sync`, or `oc apply`. Returns allow/deny with risk level, reasons, and signed evidence.

Arguments: `actor` (object), `tool` (string), `operation` (string), `environment` (string, optional), `params` (object), `context` (object).

**`get_event`** — Retrieve a previously recorded evidence event by ID.

Arguments: `event_id` (string).

## Dockerfile

The image uses a two-stage build:

1. **Builder** (`golang:1.24-alpine`) — compiles `evidra-mcp` with embedded OPA bundle
2. **Runtime** (`gcr.io/distroless/static:nonroot`) — minimal image, runs as uid 65534

No shell, no package manager, no debug tools in the final image.

## Updating the registry entry

1. Fork [docker/mcp-registry](https://github.com/docker/mcp-registry)
2. Update files in `servers/evidra/`
3. Update `source.commit` in `server.yaml` to the latest evidra commit
4. Open a PR — Docker CI validates the entry
5. After merge, the updated image appears in the catalog within 24 hours
