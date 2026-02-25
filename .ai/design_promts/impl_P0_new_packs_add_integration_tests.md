You are a senior Go + container runtime engineer working on the Evidra repository.

Context:
- evidra-mcp runs in Docker using gcr.io/distroless/static:nonroot.
- Embedded OPA bundle fallback is implemented.
- Docker must work with zero mounts and zero env vars.
- Evidence is written to a default directory (currently derived from $HOME or similar).
- P0 requirement: docker run --rm ghcr.io/.../evidra-mcp must start and handle validate without mount.

Task:
Verify and fix Docker runtime behavior so that:

1) The container runs without:
   - volume mounts
   - environment variables
2) The server starts successfully.
3) A validate call succeeds (PASS and DENY).
4) No permission errors occur when writing evidence.

STRICT REQUIREMENTS:

A) Runtime verification:
- Simulate container runtime behavior locally.
- Confirm what directory is used for evidence.
- If evidence path depends on $HOME and that path is not guaranteed writable under distroless:nonroot, fix it.

B) If a fix is required:
- Implement a minimal, safe change:
  - If running inside container (detect via UID or ENV or fallback logic),
    default evidence path should be:
      /tmp/evidra/evidence
  - Ensure directory is created with correct permissions.
- Do NOT add configuration flags.
- Do NOT introduce new dependencies.
- Do NOT change MCP protocol.
- Do NOT refactor server architecture.

C) Add a Go test:
- Simulate non-writable HOME.
- Ensure fallback evidence path works.
- Ensure validate call still returns correct decision.
- Ensure no panic occurs.

D) Add a Docker smoke test script under:
  scripts/docker_smoke_test.sh

The script must:
1) Build image locally.
2) Run:
   docker run --rm -i <image>
3) Send minimal MCP initialize (if required).
4) Send PASS validate.
5) Send DENY validate.
6) Assert:
   - allow true
   - allow false
   - rule id present
   - hint present
7) Exit non-zero on failure.

Constraints:
- No jq dependency.
- Use grep for minimal JSON validation.
- Keep script <150 lines.
- Portable bash.

E) Documentation:
- Update README Docker section if behavior changes (e.g. mention evidence stored in container /tmp).
- Add short comment in Dockerfile explaining evidence path logic.

Deliverables:
Return a git-style patch (diff) including:
- Any Go code changes
- Any new test files
- scripts/docker_smoke_test.sh
- Dockerfile comment updates (if needed)
- README updates (only if necessary)

No commentary outside the diff.