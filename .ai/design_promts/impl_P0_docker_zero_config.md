You are a DevOps engineer.

Context:
evidra-mcp now supports embedded OPA bundle fallback.
It must run with zero flags and zero configuration.

Task:
Create a minimal production-ready Dockerfile for evidra-mcp.

Requirements:

- Multi-stage build
- Stage 1: build static Go binary
- CGO disabled
- Stage 2: use gcr.io/distroless/static:nonroot
- Copy only the evidra-mcp binary
- No policy bundle copied
- No volumes required
- No environment variables required
- ENTRYPOINT must run server directly
- Run as non-root user
- Binary must rely on embedded bundle

Constraints:

- No shell in final image
- No extra files
- No policy directory copied
- No debug tools
- Keep image small

Deliver:
- Complete Dockerfile
- Short explanation of why each stage is structured this way

Promt 2
Create a Docker runtime integration verification plan for evidra-mcp.

Goal:
Prove zero-config Docker behavior.

Checklist must include:

1) Build image locally
2) Run:
   docker run --rm <image>
3) Confirm:
   - process starts
   - stderr prints "using built-in ops-v0.1 bundle"
4) Send minimal MCP validate payload via stdio
5) Confirm PASS case
6) Confirm DENY case
7) Confirm hint present
8) Confirm container exits cleanly on SIGTERM

Constraints:
- No volume mounts
- No environment variables
- Must use embedded bundle
- No reliance on local policy directory

Keep concise and procedural.

Promnt 3
You are writing a CI smoke test for Docker release.

Task:
Add a GitHub Actions step that:

1) Pulls ghcr.io/<org>/evidra-mcp:latest
2) Runs container
3) Sends a minimal validate JSON via stdio
4) Asserts that response JSON contains "allow"
5) Fails pipeline if container crashes

Constraints:
- No external test harness
- Must run in under 10 seconds
- No policy mounts
- No env vars

Output:
YAML snippet for release workflow.
