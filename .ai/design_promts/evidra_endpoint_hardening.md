You are a senior software engineer performing a controlled URL refactor.

Goal:
Standardize all public-facing URLs across this repository to the following canonical architecture:

LANDING:
https://evidra.samebits.com

MCP SERVER (HTTP JSON endpoint):
https://evidra.samebits.com/mcp

GENERAL REST API:
https://api.evidra.rest/v1

Important:
- These URLs are now the only official public endpoints.
- Remove or replace any references to:
  - https://evidra.rest
  - https://mcp.evidra.rest
  - https://api.evidra.rest (without /v1 if used as base)
  - any previous test or staging URLs
- Keep /v1 as the API base path.
- Do NOT change internal service URLs used inside containers unless they are public-facing.

Constraints:
- Do NOT modify application logic.
- Do NOT change import paths.
- Do NOT rename packages.
- Only update:
  - README files
  - documentation (*.md)
  - OpenAPI specs (servers section only)
  - environment variable defaults if they contain public URLs
  - example configs
  - curl examples
  - comments referring to public endpoints
- If URL appears in tests, update only if it is a public base URL.
- Preserve local development URLs (localhost, 127.0.0.1, internal cluster DNS).

Tasks:

1. Documentation
- Replace all outdated public URLs with canonical ones.
- Ensure README shows:
    MCP Endpoint: https://evidra.samebits.com/mcp
    REST API Base: https://api.evidra.rest/v1

2. OpenAPI / Swagger
- Update the "servers" section to:
    https://api.evidra.rest/v1

3. CLI Help Text (if present)
- Ensure help output references:
    https://evidra.samebits.com
    https://evidra.samebits.com/mcp

4. Environment Defaults
- If environment variables like:
    BASE_URL
    API_URL
    MCP_URL
  contain old domains, update to new canonical URLs.

5. Landing References
- Ensure landing references:
    API → https://api.evidra.rest/v1
    MCP → https://evidra.samebits.com/mcp

6. Remove references to:
    mcp.evidra.rest (unless internal)
    evidra.rest as primary entrypoint

Output:
- List of modified files
- Unified diff
- Summary of changes
- Confirmation that no runtime behavior was altered