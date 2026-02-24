# Product Goals Backlog

## Vision Statement
- Evidra provides a deterministic safety layer for AI-driven infrastructure changes.
- The product delivers audit-ready evidence for every decision, enabling DevOps teams to trust AI automation without sacrificing compliance.
- Focus on enforcement, explainability, and immutable logging rather than general-purpose AI gateway features.

## Priority Workstreams (ordered by impact)

### 1. Evidence & Explainability
- Immutable Evidence Records: store every PASS/FAIL with a hash-linked entry for auditors. (ongoing)
- Structured remediations: include rule IDs, hints, and reasons in CLI output so agents can self-correct.
- Explain command: expose policy hits, facts (destroy_count, namespace, kinds) in human + AI-friendly text.

### 2. Policy & Validation
- Human-readable policy profile (`policy/bundles/ops-v0.1`): split into decision aggregator + focused rule files.
  - Polished hints stored in `data.rule_hints`.
  - Rule IDs follow canonical `domain.invariant_name` format (e.g., `k8s.protected_namespace`, `ops.mass_delete`).
- Policy contract documentation for contributors: describe input schema, decision schema, hint conventions, testing strategy.
- Add data-driven rule hints so policy output stays stable even when messages change.
- Add policy smoke tests validating hits/hints/reasons.

### 3. CLI Simplicity & Core UX
- Single primary command: `evidra validate <path>` with auto-detection (terraform vs k8s).
- Minimal offline binary (`evidra`) for policy simulation/evidence inspection. Keep server binary (`evidra-mcp`) focused on policy enforcement.
- Structured CLI output: PASS/FAIL with risk, hits, hints, evidence ID; add `--json` flag.
- Root README and docs focus on one obvious workflow; advanced topics moved to `docs/advanced.md`.
- Added `SCOPE.md` to lock the v1 mission.

### 4. Registry & Execution Hygiene
- Registry purely declarative: no built-in executors; command building lives in engine/runner.
- Typed errors for tool/operation resolution and validation, with unit tests.
- Tests rely on fixture scenarios instead of demo executors (remove echo/git executors).
- `go test ./...` must pass on Go 1.23 for offline builds.

### 5. Self-Hosting & Server Configuration
- `evidra-mcp` requires explicit `--bundle`; resolve via flag > `EVIDRA_BUNDLE_PATH` env > default.
- Remove dev profile switching and demo tools from server binary.
- Provide clear server help text describing required flags and usage.
- Ensure docs describe MCP server startup plus offline tools.

## Lower-Priority Ideas (review before implementation)
- Policy export artifacts (evidence+policy+signed metadata).
- SOC2/HIPAA starter scenarios under `examples/` (only if they stay relevant after the slim focus).
- Context capture: store the triggering prompt or diff context in evidence. (Evaluate privacy/complexity.)
- CI reporting mode: record violations without blocking to train policies before enforcing.

## Essentials to Maintain
- Keep the shared validation core and evidence behavior stable; avoid new dependencies.
- Tests are a priority: add unit/integration coverage for policy, CLI, and server configuration.
- Documentation must stay concise: root README for quick start, docs/advanced.md for deeper topics.
