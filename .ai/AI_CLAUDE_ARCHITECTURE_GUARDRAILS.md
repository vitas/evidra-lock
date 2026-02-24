# Architecture Guardrails: OPA Bundle Standard

**Date:** 2026-02-24
**Author:** Claude Opus 4.6 (Principal Architect role)
**Status:** Draft (Refined 2026-02-24)
**Scope:** Architectural invariants, prohibited patterns, and enforcement strategy for the OPA Bundle architecture

---

## 1. Architectural Invariants

These invariants are unconditional. They apply to all code paths, all profiles, all environments. Violations are release-blocking and must be resolved before merge.

### INV-1: No environment branching in Go

The Go codebase must not contain any conditional logic (`if`, `switch`, `case`, ternary patterns) that inspects the value of the environment label. The environment label is an opaque `string` that flows through three points only:
1. **Input:** received from the caller (CLI flag, API parameter, MCP context)
2. **Forwarding:** placed into the OPA input document at `input.environment`
3. **Recording:** written to the evidence record as `environment_label`

No other use of the environment value is permitted in Go. The engine must not select bundles, choose configurations, adjust behavior, format output, or inject default values based on the environment value. The engine must never inject a default environment value when none is provided by the caller.

### INV-2: No environment literals in Rego

Rego policy rules must not contain string literals that represent environment names, namespace names, or any other environment-specific identifiers. Environment-specific behavior is achieved exclusively through data namespace lookups keyed by `input.environment`.

Concretely: the strings `"prod"`, `"staging"`, `"production"`, `"kube-system"`, or any other infrastructure-specific identifiers must not appear as literal values in Rego rule bodies. They belong in the data namespace (`environments.json`, `thresholds.json`).

Test files are exempt from this invariant — tests may use literal environment names as test fixtures.

### INV-3: All thresholds in data namespace

Every numeric limit, string allowlist, string denylist, and categorical control used in policy evaluation must be defined in the bundle's data namespace (JSON data files under `evidra/data/`). No threshold may be defined as:
- A Rego literal (e.g., `count > 5`)
- A Go constant
- An environment variable read at evaluation time
- A default value in a Rego rule head

The data namespace is the single source of truth for all policy-configurable parameters.

### INV-4: Single-bundle execution only

The engine accepts exactly one bundle artifact per evaluation. The bundle path must be a single filesystem path to a `.tar.gz` file. The engine must reject the following before any bundle loading occurs:
- Multiple `--bundle` flag values
- A directory path as bundle input
- A glob pattern as bundle input
- An environment variable containing multiple paths

There is no mechanism for loading, composing, layering, or merging multiple bundles. The single bundle contains the complete policy universe.

### INV-5: Bundle revision in evidence

Every evidence record produced from a bundle-based evaluation must contain a non-empty `bundle_revision` field that is an exact string match of the `.manifest.revision` from the loaded bundle. Evidence records without `bundle_revision` are permissible only during migration Phases 1 and 2 (as defined in the System Design document §9) when `LocalFileSource` is still in active use. From Phase 3 onward, every evidence record must contain a non-empty `bundle_revision`. Records without `bundle_revision` produced after Phase 3 are architectural violations.

### INV-6: Immutable artifacts

Once a bundle artifact is published with a given `revision`, that revision string must never be reused for a different artifact. The mapping from revision to artifact content is permanent and irreversible.

### INV-7: Manifest validation before evaluation

The bundle loader must validate the `.manifest` file before any policy evaluation occurs. The following conditions must each cause a hard failure (error returned, no OPA evaluation):
- `.manifest` file is absent from the archive
- `.manifest` file is not valid JSON
- `revision` field is absent, null, or empty string
- `roots` field is absent, null, or not an array
- `roots` does not contain the string `"evidra"`
- `metadata.profile_name` is absent, null, or empty string

There must be no fallback to a default manifest, no inference of roots from directory structure, no generation of a synthetic revision, and no inference of profile name from filename or directory. A bundle without a valid manifest is not a valid bundle.

### INV-8: Deterministic output ordering

All multi-value decision fields (`Hits`, `Hints`, `Reasons`) must be sorted lexicographically (ascending UTF-8 byte order) by the engine before being returned to callers or recorded in evidence. This sorting must be unconditional — applied on every evaluation, not only during testing. No decision output field may depend on Go map iteration order or OPA set iteration order.

### INV-9: Policy identity authority

When `BundleRevision` is non-empty in an evidence record, `BundleRevision` is the sole authoritative policy identity. `PolicyRef` must not be used for audit, replay, provenance, or release lookup in this case. `PolicyRef` is informational only — it serves as a content integrity check, not as an identity. No evidence consumer, reporting tool, or external integration may treat `PolicyRef` as policy identity when `BundleRevision` is present. See System Design §8 for the complete authority table.

### INV-10: Unknown environment is bundle author responsibility

The engine must not implement fail-open or fail-closed semantics for unknown environments. The engine must not reject evaluations where the environment label is unknown. The engine must not log warnings about unknown environments. The bundle author is solely responsible for defining behavior when `input.environment` does not match any key in the data namespace. Every bundle must include test coverage for the unknown-environment path. See System Design §4 for the full contract.

### INV-11: No semantic branching on PolicyRef() value

**INVARIANT: Engine MUST NOT branch on the semantic meaning of `PolicyRef()` value.**

`PolicyRef()` returns an opaque string. The engine must treat it as a passive identifier only — a value to be stored, forwarded, and recorded, never inspected. The engine must not inspect, parse, prefix-check, or pattern-match the value. The engine must not branch on whether the value looks like a SHA-256 hash, a manifest revision, or any other recognizable format.

The following conditional patterns are prohibited on `PolicyRef()` values:
- `strings.HasPrefix` / `strings.HasSuffix` / `strings.Contains`
- Regular expression matching
- Length-based inference (e.g., `len(ref) == 64` to detect a hash)
- Format detection of any kind

Policy identity authority is determined exclusively by `BundleRevision` presence (non-empty vs empty), not by the format or content of `PolicyRef()`. See System Design §6 for the full invariant statement and §8 for the policy identity authority table.

Any branching on `PolicyRef()` value is release-blocking.

---

## 2. Prohibited Patterns

Each pattern below is an anti-pattern that violates one or more architectural invariants. Code reviews and automated checks must reject these patterns.

### ANTI-1: Hardcoded thresholds

**What it looks like:**
- Rego: `count > 5` instead of `count > data.evidra.data.thresholds[env].mass_delete_max`
- Go: `const maxDeleteCount = 5` used in evaluation logic
- Rego: `default mass_delete_max = 5` as a rule default

**Why it is prohibited:** Hardcoded thresholds bypass the data namespace, making them invisible to bundle configuration and impossible to vary by environment without code changes. They violate INV-3.

**Correct alternative:** All thresholds in JSON data files within the bundle, accessed via data namespace lookups.

### ANTI-2: Inline environment checks

**What it looks like:**
- Rego: `input.environment == "prod"` or `action_namespace(action) == "kube-system"`
- Go: `if env == "prod" { ... }`
- Rego: `default is_production = false` with `is_production { input.environment == "production" }`

**Why it is prohibited:** Inline environment checks create an implicit enumeration of known environments. Adding a new environment requires modifying Rego or Go code rather than adding a data entry. They violate INV-1 and INV-2.

**Correct alternative:** Rego rules look up environment-specific configuration from the data namespace using `input.environment` as a key. The set of valid environments is defined entirely by the data.

### ANTI-3: Silent fallback bundles

**What it looks like:**
- Loading a "default" bundle when the specified bundle is not found
- Automatically falling back to `LocalFileSource` when bundle loading fails
- Using a bundled-in "base policy" that activates when no bundle is provided
- Applying a "minimum policy" when the manifest is invalid
- Trying a secondary bundle path when the primary path fails

**Why it is prohibited:** Silent fallbacks mask configuration errors and break determinism. If the specified bundle is invalid or missing, the evaluation must fail with an error. The operator must know that their policy did not load, not receive a decision from an unexpected fallback policy. Violates INV-4 and INV-7.

**Clarification:** Explicitly selecting `LocalFileSource` via `--policy`/`--data` flags during the migration period is not a silent fallback — it is an explicit mode choice. The prohibited pattern is the engine silently switching from bundle mode to file mode when bundle loading fails.

**Correct alternative:** Hard failure with a descriptive error message identifying the specific failure (missing file, invalid manifest, parse error, I/O error).

### ANTI-4: Multi-bundle creep

**What it looks like:**
- Accepting a `--bundle` flag multiple times
- Accepting a directory of bundles
- Implementing a "bundle search path"
- A "base bundle + override bundle" composition model
- Separate bundles for "rules" and "data"

**Why it is prohibited:** Multi-bundle composition introduces merge semantics, precedence rules, conflict resolution, and non-obvious interaction effects. It destroys the property that the single bundle is the complete policy universe. It makes deterministic replay dependent on bundle ordering. Violates INV-4.

**Correct alternative:** All policy and data for a given evaluation context are packaged in a single bundle. If different environments need different thresholds, those thresholds coexist as environment-keyed entries in the same bundle's data namespace.

### ANTI-5: Engine-level environment defaults

**What it looks like:**
- Go: `if environment == "" { environment = "default" }`
- Go: defaulting to a "safe" environment when none is provided
- Rego: `default environment = "base"`

**Why it is prohibited:** Default environment values create an invisible fallback that produces decisions without the caller's explicit intent. If the caller does not provide an environment, the behavior should be determined by the policy's handling of missing data (which the policy author controls), not by the engine. Violates INV-1.

**Correct alternative:** The engine passes the environment label as-is, including empty string. The policy's data namespace lookups determine what happens when the key is missing or empty.

### ANTI-6: Data split across bundle and runtime

**What it looks like:**
- Some thresholds in the bundle's data namespace, others in environment variables
- OPA `data.evidra.data.thresholds` supplemented by Go-injected runtime data
- Evidence metadata fields used as implicit policy inputs

**Why it is prohibited:** Splitting policy data across the bundle and the runtime breaks the single-source-of-truth property. The bundle alone must fully determine policy behavior for a given input and environment. Violates INV-3.

**Correct alternative:** All policy data in the bundle. Runtime provides only the input document and the environment label.

### ANTI-7: Non-deterministic output ordering

**What it looks like:**
- Returning `Hits`, `Hints`, or `Reasons` in OPA set iteration order (non-deterministic)
- Using Go map iteration order to populate decision output arrays
- Sorting only during tests but not during production evaluation

**Why it is prohibited:** Non-deterministic output ordering makes evidence records non-reproducible, breaks byte-identical determinism assertions, and produces flaky tests. Violates INV-8.

**Correct alternative:** The engine must sort all multi-value decision fields lexicographically (ascending UTF-8 byte order) unconditionally after every OPA evaluation, before returning the decision to any caller.

---

## 3. CI Enforcement Strategy

### Automated linting rules

| Check | Tool | Scope | Blocks merge |
|---|---|---|---|
| Rego syntax validity | `opa check` | All `.rego` files in `policy/bundles/` | Yes |
| Rego test pass | `opa test` | All test files in `policy/bundles/*/tests/` | Yes |
| No environment literals in Rego | `grep` / custom lint | All non-test `.rego` files | Yes |
| No numeric literals in deny/warn rules | `grep` / Regal | All rule `.rego` files | Yes |
| No Go environment branching | `grep` / AST analysis | All `.go` files in `pkg/`, `cmd/`, `internal/` | Yes |
| Manifest presence | File existence check | `policy/bundles/*/.manifest` | Yes |
| Manifest `revision` field present | JSON validation | `.manifest` files | Yes |
| Manifest `roots` contains `"evidra"` | JSON validation | `.manifest` files | Yes |
| Namespace validation | Directory structure check | All Rego packages are under `evidra.policy` | Yes |
| Data namespace validation | Directory structure check | All JSON data files are under `evidra/data/` | Yes |
| No imports from forbidden packages | `go vet` / import analysis | Dependency boundary checks per invariants | Yes |

### Rego lint patterns to detect

The CI pipeline must scan non-test `.rego` files for these patterns and reject matches:

| Pattern | Regex (approximate) | Violation |
|---|---|---|
| Environment literal comparison | `==\s*"(prod\|staging\|production\|dev\|test)"` | INV-2 |
| Namespace literal in deny/warn | `==\s*"(kube-system\|default\|kube-public)"` in rule bodies | INV-2 |
| Numeric literal in comparison | `[><=]+\s*\d+` in deny/warn rule bodies (not in test files) | INV-3 |
| Default threshold value | `default\s+\w+\s*=\s*\d+` | INV-3 |

**Static analysis method preference:** Regex-based pattern matching is a baseline detection mechanism suitable for initial CI implementation. It is not sufficient as the sole long-term enforcement strategy due to inherent false-positive and false-negative risks (e.g., regex cannot distinguish a string literal in a comment from one in a rule body).

Where feasible, CI checks must operate on parsed ASTs rather than raw text:
- **Rego:** Use `opa parse --format json` to obtain the Rego AST. Detect string literals in rule bodies by inspecting term nodes. This eliminates false positives from comments and string constants used in non-comparison contexts.
- **Go:** Use `go/ast` or `go/analysis` framework to detect conditional expressions referencing the environment variable. This eliminates false positives from comments, documentation strings, and unrelated variables named "environment."

The transition from regex to AST-based checks must be completed before migration Phase 3 (bundle-only runtime path). Until AST-based checks are in place, regex checks remain mandatory and release-blocking.

**Scope and false-positive handling:** The environment literal and numeric literal checks apply only to rule files under `policy/bundles/*/evidra/policy/rules/`. Helper files (`defaults.rego`, `decision.rego`) and test files (`tests/`) are excluded from the numeric literal check. If a legitimate false positive occurs, it must be suppressed with an inline annotation comment (`# guardrail:allow <pattern-id>`) and documented in the pull request with justification. Inline suppressions are reviewed as part of the PR review checklist. Suppressions must be narrow — they apply to the specific line, not to the file or directory.

### Go lint patterns to detect

| Pattern | Detection method | Violation |
|---|---|---|
| `if.*environment.*==` or `switch.*environment` | `grep` on Go source | INV-1 |
| `const.*Env.*=` (environment enum constants) | `grep` on Go source | INV-1 |
| Import of `pkg/bundlesource` from `pkg/evidence` | Import graph analysis | Dependency boundary |
| Import of `pkg/policy` from `pkg/evidence` | Import graph analysis | Dependency boundary |
| Import of `pkg/validate` from `pkg/bundlesource` | Import graph analysis | Dependency boundary |
| Conditional logic referencing `PolicyRef` return value | Grep/AST scan for `PolicyRef` in conditional expressions, `strings.Has*`, regex, or length checks | INV-11 |

### Review checklist

Every pull request that modifies policy, engine, or evidence code must be reviewed against this checklist:

1. Does the change preserve single-bundle execution? (No multi-bundle loading introduced)
2. Are all policy-configurable values sourced from the data namespace? (No new thresholds in Rego or Go)
3. Is the environment label still opaque? (No new Go branching or Rego literals based on environment)
4. Does the evidence schema remain sufficient for deterministic replay? (bundle_revision, profile_name, environment_label, input_hash all present)
5. Does the bundle manifest remain valid? (revision non-empty, roots include "evidra")
6. Are dependency boundaries preserved? (No forbidden imports introduced)
7. If a new rule is added, does it follow the data-driven pattern? (Lookup-based, not literal-based)

### Manifest validation in CI

The CI pipeline must validate every `.manifest` file on every commit that modifies files under `policy/bundles/`:

| Check | Requirement |
|---|---|
| File is valid JSON | Parse succeeds |
| `revision` field exists | Non-null, non-empty string |
| `roots` field exists | Non-null array |
| `roots` contains `"evidra"` | Exact string match |
| No extra roots | `roots` has exactly one element (MVP: only `"evidra"`) |

### Namespace validation in CI

For each profile directory under `policy/bundles/`:

| Check | Requirement |
|---|---|
| All `.rego` files are under `evidra/policy/` | No Rego files at bundle root or under `evidra/data/` |
| All `.json` data files are under `evidra/data/` | No data files under `evidra/policy/` |
| All Rego `package` declarations start with `evidra.policy` | Parse each `.rego` file and check package path |
| No files outside `evidra/` and `.manifest` | Archive does not contain unexpected paths |

---

## 4. Drift Detection Model

Drift is the gradual, accidental violation of architectural invariants through incremental changes that individually seem harmless but collectively erode the architecture.

### Detection signals

| Signal | Invariant at risk | Detection method |
|---|---|---|
| New Go conditional on environment value | INV-1 | Grep/AST scan for `environment` in conditional expressions |
| New string literal in Rego rule body matching infrastructure names | INV-2 | Grep for known infrastructure patterns (`prod`, `staging`, `kube-*`) |
| New numeric literal in Rego deny/warn rule body | INV-3 | Grep for `[><=]\s*\d+` in rule files |
| Evidence record with empty `bundle_revision` after migration Phase 3 | INV-5 | Query evidence store for records where `bundle_revision` is empty |
| Published artifact whose manifest revision does not match filename | INV-6 | Release pipeline integrity check |
| New import from a forbidden package | Dependency boundaries | Import graph analysis via `go list -json ./...` |
| New `--bundle` flag accepting multiple values or a directory | INV-4 | Code review + flag definition analysis |
| New Rego `default` statement setting a numeric value | INV-3 | Grep for `default\s+\w+\s*=\s*\d` |
| Bundle artifact checksum mismatch on rebuild | Determinism | Rebuild from tag, compare checksums |
| Decision output arrays not sorted | INV-8 | Determinism test with multi-rule input, verify sorted output |
| Unsorted slice assignment to decision fields in Go | INV-8 | Code review + grep for direct OPA result-to-slice assignment without sort |
| Conditional logic referencing `PolicyRef` return value | INV-11 | Grep/AST scan for `PolicyRef` in conditional expressions, `strings.Has*`/regex/length checks on PolicyRef values |

### Detection cadence

| Cadence | Trigger mechanism | Checks performed |
|---|---|---|
| **Every pull request** | CI pipeline (automated, mandatory) | All CI lint rules: Rego syntax, Go patterns, manifest validation, namespace validation, import graph analysis |
| **Every release** | Release pipeline (automated, mandatory) | Bundle integrity (manifest-filename-tag consistency), deterministic rebuild verification, smoke test evaluation, checksum generation |
| **Weekly** | Scheduled CI job (cron, automated) | Full evidence store scan for records missing `bundle_revision` (post-migration Phase 3). Full Go AST scan for environment branching. Full Rego scan for literal patterns. Expired exception scan. |
| **Quarterly** | Calendar-triggered review (manual, documented) | Dependency graph visualization and review. Data namespace coverage audit. Rule-to-data complexity ratio analysis. Results recorded in `ai/AI_DECISIONS.md`. |

Weekly and quarterly scans must produce machine-readable reports. Weekly scan failures must create tracked issues automatically. Quarterly review findings must be documented before the next release.

### Remediation policy

| Severity | Condition | Response |
|---|---|---|
| **Blocking** | Any invariant violation detected in CI | PR cannot merge until resolved |
| **Blocking** | Release artifact fails integrity checks | Release is not published |
| **High** | Drift detected by scheduled scan | Issue filed, fix required within 5 business days |
| **Medium** | Architecture review finds structural concerns | Documented in decision log, fix planned for next milestone |

### Temporary exceptions

In exceptional circumstances, an invariant may be temporarily violated. All of the following requirements must be met:
- Written justification in `ai/AI_DECISIONS.md` identifying the specific invariant (by INV-number) being violated and the reason
- Explicit expiration date (maximum 30 calendar days from the date of the exception)
- Named owner (individual, not team) responsible for resolving the exception before expiration
- CI annotation that suppresses the specific check for the specific file and line only (no broad suppressions)
- Pull request creating the exception must be approved by at least one reviewer who is not the exception owner

The following invariants are unconditionally non-exemptable — no temporary exception may be granted:
- **INV-4** (single-bundle execution)
- **INV-6** (immutable artifacts)
- **INV-7** (manifest validation before evaluation)
- **INV-11** (no semantic branching on PolicyRef)

Expired exceptions without resolution are treated as blocking violations. The weekly scheduled scan must detect expired exceptions and create blocking issues automatically. An expired exception blocks all releases until resolved.
