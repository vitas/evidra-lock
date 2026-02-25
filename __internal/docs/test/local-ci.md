# Running CI Locally

## Prerequisites

- **Docker** — must be running (`docker info` to verify)
- **act** — local GitHub Actions runner

```sh
brew install act
```

### First-run setup

On first run `act` prompts for a runner image. To avoid the interactive prompt,
create the config file upfront:

```sh
mkdir -p ~/Library/Application\ Support/act
echo '-P ubuntu-latest=catthehacker/ubuntu:act-latest' \
  > ~/Library/Application\ Support/act/actrc
```

This uses the "medium" image (~500 MB) which is sufficient for Docker builds.

## Workflows

| Workflow | File | Purpose |
|---|---|---|
| Docker Smoke (local) | `.github/workflows/docker-smoke-local.yml` | Build + smoke test only — no GHCR auth needed |
| Release | `.github/workflows/release.yml` | Full release: GHCR push + GoReleaser (requires tokens) |

## Usage

### Docker smoke test (recommended for local dev)

```sh
act push --workflows .github/workflows/docker-smoke-local.yml
```

This builds the Docker image and runs the MCP smoke test (initialize → validate → check response).

Expected output on success:

```
[Docker Smoke (local)/docker-smoke]   | using built-in ops-v0.1 bundle
[Docker Smoke (local)/docker-smoke]   | PASS: smoke test ok
[Docker Smoke (local)/docker-smoke]   ✅  Success - Main Smoke test
[Docker Smoke (local)/docker-smoke] 🏁  Job succeeded
```

### Release workflow (dry-run)

```sh
# List jobs
act -l --workflows .github/workflows/release.yml

# Dry-run (no execution)
act push --job docker -n --workflows .github/workflows/release.yml

# Simulate a tag push (GHCR login will fail without a real token)
act push --job docker --workflows .github/workflows/release.yml \
  --eventpath /dev/stdin <<< '{"ref":"refs/tags/v0.1.0"}'
```

## Limitations

- `act` cannot push to GHCR without a real token — `docker/login-action` will fail.
  Use `docker-smoke-local.yml` to test build + smoke without auth.
- The medium runner image (~500 MB) is pulled on first run; subsequent runs use cache.
- Nested Docker (docker-in-docker) requires the host Docker socket to be accessible.
