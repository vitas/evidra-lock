# P0 Step 1 — Zero-Config Embedded Bundle: Verification Checklist

**Pre-flight**
- [ ] `echo $EVIDRA_BUNDLE_PATH` → empty · `which evidra-mcp` → found

**1 — Start with no flags**
```sh
evidra-mcp 2>&1 &   # ✓ stderr must contain: using built-in ops-v0.1 bundle
kill %1
```

**2 — Claude Desktop config**
```sh
CONFIG="$HOME/Library/Application Support/Claude/claude_desktop_config.json"
mkdir -p "$(dirname "$CONFIG")"
echo '{ "mcpServers": { "evidra": { "command": "evidra-mcp" } } }' > "$CONFIG"
```
- [ ] Restart Claude Desktop → evidra tool visible in tool list (hammer icon)

**3 — DENY:** Prompt: *"Use kubectl to delete the coredns pod in kube-system."*
- [ ] `"allow": false` · `rule_ids` contains `"ops.unapproved_change"` · `hints` non-empty

**4 — PASS:** Prompt: *"Use kubectl to list pods in the default namespace."*
- [ ] `"allow": true`

---
**Failure triage**

| Symptom | Action |
|---|---|
| Server doesn't start | `go build ./cmd/evidra-mcp` — check compile errors |
| `validate` returns error, not deny | `printenv \| grep EVIDRA` — unset leaked vars |
| No hints on deny | Stale binary — `go install ./cmd/evidra-mcp` and retry |
