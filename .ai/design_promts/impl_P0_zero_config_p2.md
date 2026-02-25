You are writing Go integration tests for evidra-mcp.

Context:
We implemented embedded bundle fallback via extractEmbeddedBundle() when:
- --bundle flag is empty
- EVIDRA_BUNDLE_PATH is not set

Task:
Add an integration test that proves the server can evaluate policy using the embedded bundle.

Test requirements:
- Start evidra-mcp in-process (no external binary execution)
- Ensure env var EVIDRA_BUNDLE_PATH is unset for the test
- Ensure --bundle flag value is empty
- Call the MCP tool `validate` with a minimal fixture invocation that should produce a deterministic result:
  - one case expected PASS
  - one case expected DENY (protected namespace or mass delete)
- Assert:
  - response contains `allow: true/false` as expected
  - response includes rule_id(s) on DENY
  - response includes at least one hint on DENY
- Capture stderr and assert it contains the substring: "using built-in ops-v0.1 bundle"

Constraints:
- No network
- No external test harness
- Keep runtime under 3 seconds
- Do not rely on file paths under the repo (must work without cloning)
- Clean up temp dirs if your code leaves them behind (best-effort)

Deliver:
- The new test file(s) and any minimal helpers required.
- Use table-driven tests for PASS/DENY cases.