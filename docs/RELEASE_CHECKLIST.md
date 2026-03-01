# Pre-release alignment checklist

Before tagging a release, verify:

- [ ] baseline/ops profile gating works (or removed from landing/README)
- [ ] MCP tool description matches actual supported tools in
      ops.destructive_operations
- [ ] CI example (`terraform show -json | evidra validate`) matches
      actual CLI interface
- [ ] Evidence claims match implementation:
      - SHA-256 hash chain: implemented
      - Action digests: not yet (don't claim "per-action digest")
      - Canonical JSON: not yet (don't claim "deterministic digest")
- [ ] Install methods listed (brew, docker, go install) actually work
- [ ] `HOMEBREW_TAP_TOKEN` can access `samebits/homebrew-tap` (private repo) with `Contents: Read and write`
- [ ] "Ops layer" copy matches the current bundle rule set
