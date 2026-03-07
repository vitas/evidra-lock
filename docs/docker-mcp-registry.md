# MCP Registry Publications

Evidra is published to two MCP registries, providing both local (Docker) and hosted installation options.

---

## 1. Docker MCP Registry

Published to the [Docker MCP Registry](https://github.com/docker/mcp-registry), making Evidra available as a one-click MCP server in Docker Desktop.

### Image

```
mcp/evidra
```

Docker builds and hosts the image from the project Dockerfile. The image is signed, includes provenance attestation and SBOM.

### Zero-config

The Docker image works out of the box with no configuration:

- **Embedded OPA bundle** — the `ops-v0.1` policy bundle is compiled into the binary via `go:embed` and extracted to `~/.evidra/bundles/ops-v0.1/` on first run.
- **Stdio transport** — the ENTRYPOINT runs `evidra-mcp` directly, which speaks MCP over stdin/stdout.
- **Deny cache enabled** — `EVIDRA_DENY_CACHE=true` is set by default in the registry config to prevent agent deny-loops.
- **No secrets required** — runs in offline mode (local policy evaluation, no API calls).

### Docker Desktop usage

After installing from the Docker MCP catalog, Evidra appears as an MCP server in Docker Desktop. AI agents connected through Docker Desktop MCP can call `validate` before destructive infrastructure operations.

### Registry files

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
  icon: https://www.samebits.com/evidra-icon.png
source:
  project: https://github.com/vitas/evidra-lock
run:
  env:
    EVIDRA_DENY_CACHE: "true"
```

### Validation and build

```bash
# Install go-task
brew install go-task

# Validate the registry entry
cd ~/git/mcp-registry && task validate -- --name evidra

# Build the image (Docker pulls source from the pinned commit)
cd ~/git/mcp-registry && task build -- --tools evidra
```

### Updating the Docker registry entry

1. Fork [docker/mcp-registry](https://github.com/docker/mcp-registry)
2. Update files in `servers/evidra/`
3. Update `source.commit` in `server.yaml` to the latest evidra commit
4. Run `task validate -- --name evidra` and `task build -- --tools evidra`
5. Open a PR — Docker CI validates the entry
6. After merge, the updated image appears in the catalog within 24 hours

---

## 2. MCP Registry (modelcontextprotocol.io)

Published to the official [MCP Registry](https://registry.modelcontextprotocol.io) as `io.github.vitas/evidra-lock`. Supports both local OCI installation and hosted remote access.

### Registry entry

```
https://registry.modelcontextprotocol.io/v0.1/servers?search=io.github.vitas/evidra-lock
```

### server.json

The `server.json` file in the project root defines the registry metadata:

```json
{
  "name": "io.github.vitas/evidra-lock",
  "title": "Evidra",
  "description": "Fail-closed policy guardrails for AI agents running kubectl, terraform, helm, and argocd.",
  "version": "0.2.0",
  "packages": [
    {
      "registryType": "oci",
      "identifier": "ghcr.io/vitas/evidra-lock-mcp:0.2.0",
      "transport": { "type": "stdio" }
    }
  ],
  "remotes": [
    {
      "type": "streamable-http",
      "url": "https://evidra.samebits.com/mcp"
    }
  ]
}
```

### OCI ownership verification

The Dockerfile includes a LABEL that the MCP Registry uses to verify ownership:

```dockerfile
LABEL io.modelcontextprotocol.server.name="io.github.vitas/evidra-lock"
```

### GHCR image

The OCI package is published to GitHub Container Registry:

```
ghcr.io/vitas/evidra-lock-mcp:0.2.0
```

The package must be **public** for the MCP Registry to verify it.

### Publishing steps

Prerequisites: `mcp-publisher` CLI and a GitHub account.

```bash
# 1. Install mcp-publisher
brew install mcp-publisher

# 2. Build and push the Docker image to GHCR
docker build -t ghcr.io/vitas/evidra-lock-mcp:VERSION -f Dockerfile .
echo "GITHUB_TOKEN" | docker login ghcr.io -u vitas --password-stdin
docker push ghcr.io/vitas/evidra-lock-mcp:VERSION

# 3. Ensure the GHCR package is public
# Go to: https://github.com/users/vitas/packages/container/evidra-mcp/settings
# Danger Zone → Change visibility → Public

# 4. Authenticate with MCP Registry
mcp-publisher login github
# Follow the device flow: visit https://github.com/login/device and enter the code

# 5. Update version in server.json, then publish
mcp-publisher publish

# 6. Verify
curl -s "https://registry.modelcontextprotocol.io/v0.1/servers?search=io.github.vitas/evidra-lock" | jq .
```

### Updating the MCP Registry entry

1. Build and push a new image tag to GHCR (`ghcr.io/vitas/evidra-lock-mcp:NEW_VERSION`)
2. Ensure the new tag is public
3. Update `version` and `packages[0].identifier` in `server.json`
4. Run `mcp-publisher publish`

---

## Shared Dockerfile

Both registries use the same `Dockerfile`. The OCI verification LABEL is harmless metadata that Docker MCP Registry ignores.

### Tools

Both registries expose the same two MCP tools:

**`validate`** — Evaluate OPA policy against a proposed infrastructure operation. Call before `kubectl apply`, `terraform apply`, `helm install`, `argocd sync`, or `oc apply`. Returns allow/deny with risk level, reasons, and signed evidence.

Arguments: `actor` (object), `tool` (string), `operation` (string), `environment` (string, optional), `params` (object), `context` (object).

**`get_event`** — Retrieve a previously recorded evidence event by ID.

Arguments: `event_id` (string).

### Image architecture

Two-stage build:

1. **Builder** (`golang:1.24-alpine`) — compiles `evidra-mcp` with embedded OPA bundle
2. **Runtime** (`gcr.io/distroless/static:nonroot`) — minimal image, runs as uid 65534

No shell, no package manager, no debug tools in the final image.
