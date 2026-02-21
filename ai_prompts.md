## 2026-02-21 14:23:14
[TASK] Implement formal "Guarded Mode" for Evidra-MCP (enforcement hardening + bypass prevention + documentation) + append-only prompt log

[MANDATORY FIRST STEP — PROMPT LOGGING]
Append THIS ENTIRE PROMPT verbatim to ai_prompts.md:

---
## <YYYY-MM-DD HH:MM:SS>
<full prompt text>
---

[CONTEXT]
Evidra-MCP currently relies on proper integration to intercept tool execution. We need a formalized "Guarded Mode" to strengthen enforcement assumptions and reduce bypass risk in regulated environments.

Goal:
Make Guarded Mode explicit, enforceable, and documented.

--------------------------------------------------
PART A — CLI MODE
--------------------------------------------------

1) Add CLI flag:
   --guarded

2) When Guarded Mode is enabled:
   - Only registered tools may execute
   - No fallback execution paths allowed
   - Direct shell execution through the gateway is forbidden
   - Any unknown operation must return explicit denial

3) Log in startup:
   "Running in GUARDED MODE (strict enforcement)"

--------------------------------------------------
PART B — STRICT TOOL REGISTRY ENFORCEMENT
--------------------------------------------------

1) If tool not found in registry:
   - Return DENIED
   - Record evidence with reason: "tool_not_registered"
   - Exit non-zero

2) Remove any implicit execution path.
   No:
     - dynamic command passthrough
     - fallback to shell
     - generic exec wrapper

3) Ensure policy evaluation happens BEFORE any execution.

--------------------------------------------------
PART C — BYPASS DETECTION
--------------------------------------------------

Implement minimal detection logic:

If invocation attempts:
   - direct shell commands
   - arbitrary script execution
   - non-registered binary path

Then:
   - deny
   - evidence record with violation_type: "bypass_attempt"

Add test coverage for:
   - direct shell bypass attempt
   - unregistered tool
   - registered tool success path

--------------------------------------------------
PART D — SECURITY MODEL DOCUMENTATION
--------------------------------------------------

Create:
  docs/SECURITY_MODEL.md

Include:

1) Enforcement assumptions
   - All tool execution must go through Evidra-MCP
   - Agents must not have direct shell access
   - Guarded Mode recommended for production

2) Tamper-evident scope
   - Evidence chain protects against accidental or partial modification
   - Full filesystem compromise not prevented
   - Future anchoring roadmap

3) Known bypass vectors
   - Direct OS access
   - Separate execution channels
   - Manual evidence rewriting if attacker controls host

4) Recommended deployment pattern
   - Run gateway in isolated container
   - Remove shell from agent containers
   - Use network isolation
   - Export evidence off-host

--------------------------------------------------
PART E — README UPDATE
--------------------------------------------------

Add section:
## Guarded Mode

Explain:
- What it does
- When to use it
- Production recommendation
- Limitations

--------------------------------------------------
PART F — TESTS
--------------------------------------------------

Add tests for:
- Guarded Mode denies unregistered tool
- Guarded Mode denies shell passthrough
- Normal mode still works for development
- Evidence is recorded for denials

--------------------------------------------------
CONSTRAINTS
--------------------------------------------------

- Do not break current architecture.
- Keep minimal complexity.
- No heavy new dependencies.
- Preserve evidence chain logic.

--------------------------------------------------
DEFINITION OF DONE
--------------------------------------------------

- --guarded flag implemented.
- Strict enforcement behavior validated by tests.
- SECURITY_MODEL.md added.
- README updated.
- No fallback execution paths remain.
- Build and tests pass.
