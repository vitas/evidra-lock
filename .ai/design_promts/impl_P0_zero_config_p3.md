Create a concise manual verification checklist for P0 Step 1 (zero-config embedded bundle).

Checklist must be runnable on a clean macOS machine.

Include:
1) Verify no EVIDRA_BUNDLE_PATH is set
2) Run `evidra-mcp` with zero flags
3) Confirm stderr shows "using built-in ops-v0.1 bundle"
4) Confirm Claude Desktop can connect using a minimal config snippet with only "command"
5) Trigger a known DENY prompt (kube-system delete) and confirm:
   - allow=false
   - hint present
   - rule id present
6) Trigger a known PASS prompt and confirm allow=true

Include a "failure triage" section:
- If server doesn’t start
- If validate returns error
- If no hints are returned

Keep it under 30 lines total.