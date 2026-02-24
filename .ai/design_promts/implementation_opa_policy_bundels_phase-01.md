You are a senior engineer refactoring an existing Evidra policy repository to comply with the new architecture spec.

Primary references (must follow exactly):
- .ai/AI_CLAUDE_SYSTEM_DESIGN_OPA_BUNDLES.md
- .ai/AI_CLAUDE_ARCHITECTURE_GUARDRAILS.md

Task:
Convert the current policy packs/policies/data layout into OPA-compliant bundles per the new specification.
This is a direct refactor (no backward compatibility required).

Hard constraints:
- OPA bundle is the only policy artifact.
- Exactly one bundle per execution (no composition).
- Bundle data documents must be OPA-loadable (directory-based data.json/data.yaml only).
- .manifest required at bundle root with:
  - revision (non-empty)
  - roots (MVP: exactly ["evidra"])
  - metadata.profile_name (authoritative; must match CLI --profile)
- Tunables live under data.evidra.data.params (per spec).
- No environment enums; environment is an opaque string passed in input context.
- No environment literals in Rego.
- No semantic branching on PolicyRef() anywhere.
- Deterministic output required (sorting + canonical JSON rules).
- No SaaS, no MCP, no multi-bundle.

Deliverables:
1) Updated bundle directory structure under:
   policies/bundles/<profile>/
2) Converted Rego modules under:
   <bundle>/evidra/policy/...
3) Converted data docs under:
   <bundle>/evidra/data/<name>/data.json
   specifically:
   - evidra/data/params/data.json      -> data.evidra.data.params
   - evidra/data/rule_hints/data.json  -> data.evidra.data.rule_hints (if required by spec)
4) Remove legacy file names/paths that OPA would ignore (e.g., params.json at arbitrary paths).
5) A short conversion report (as markdown output in chat) listing:
   - what was moved/renamed
   - what was deleted
   - any rule IDs renamed
   - any required follow-ups

Process (must follow in order):

PHASE 1 — Inventory (no changes yet)
- Enumerate all existing policy packs / directories.
- List all existing Rego packages and their current paths.
- List all existing data/config files and their current paths and top-level keys.
- Identify any current “thresholds/environments” style configs or any ad-hoc config maps.
- Identify current rule identifiers (rule_id strings) and where they are produced.

PHASE 2 — Choose bundle profiles and naming
- Define the set of profiles (bundle names) we will ship now (e.g., ops-v0.1, gitops-v0.1).
- For each profile, define metadata.profile_name (must equal the directory name used by --profile).
- Confirm roots will be exactly ["evidra"] for all bundles (MVP strict).

PHASE 3 — Restructure into OPA bundle layout
For each bundle:
- Create: policies/bundles/<profile>/.manifest
- Move Rego into: policies/bundles/<profile>/evidra/policy/...
- Move data into: policies/bundles/<profile>/evidra/data/<namespace>/data.json
- Ensure no extra wrapper directory inside the bundle that would break OPA loading.
- Ensure any “data” files are named data.json (or data.yaml) and placed so OPA maps them into correct namespaces.

PHASE 4 — Convert configuration to params namespace
- Consolidate ALL tunables into:
  data.evidra.data.params (in evidra/data/params/data.json)
- Use stable param keys (no env/severity/ordinals/version).
- If the spec mandates a param structure (e.g., by_env/default), follow it exactly.
- Remove/replace any legacy namespaces:
  - data.evidra.data.thresholds
  - data.evidra.data.environments
- Update Rego to read tunables only from params namespace (no numeric tunable literals in rule bodies).

PHASE 5 — Rule ID refactor (if old IDs exist)
- Replace any legacy semantic IDs like POL-PROD-01, WARN-AUTO-01 with stable rule IDs (domain.invariant_name).
- Ensure rule_id does not encode env/severity/ordinals.
- Update rule outputs and any references accordingly.
- Ensure violations include required fields per spec (rule_id, severity from data if specified, message, hint optional).
- Ensure output ordering determinism is preserved.

PHASE 6 — Required shared helper(s)
- If the spec mandates a shared helper (e.g., resolve_param), ensure:
  - correct name
  - correct location file path
  - correct package
- Ensure all rules use the helper consistently if required by spec.
- No code/pseudocode in report, but implement in repo.

PHASE 7 — Validate OPA loading semantics
- Verify that the new data layout is actually loaded by OPA:
  - params visible as data.evidra.data.params
  - rule_hints visible as data.evidra.data.rule_hints
- Ensure no data depends on filenames that OPA ignores.

PHASE 8 — Clean up
- Delete legacy pack layouts that are no longer used.
- Delete files that will be ignored by OPA or violate guardrails.
- Update README/examples if they reference old paths or IDs.

PHASE 9 — Tests and CI guardrail alignment
- Update or add tests so they pass with the new bundle layout.
- Ensure CI checks required by guardrails will pass:
  - forbidden namespaces not present
  - forbidden env literals not present
  - no ad-hoc data file naming
  - deterministic outputs / ordering tests can be run

Output format (in your response):
1) “Conversion Plan” (bulleted)
2) “Bundle Layout Result” (tree per bundle)
3) “Config/Namespace Changes” (what moved where)
4) “Rule ID Changes” (if any)
5) “Risk Checklist” (what to double-check)
Do NOT include code blocks unless explicitly asked.