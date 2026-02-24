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
1. **Input:** received from the caller (CLI flag, API parameter)
2. **Forwarding:** placed into the OPA input document at `input.environment`
3. **Recording:** written to the evidence record as `environment_label`

No other use of the environment value is permitted in Go. The engine must not select bundles, choose configurations, adjust behavior, format output, or inject default values based on the environment value. The engine must never inject a default environment value when none is provided by the caller.

### INV-2: No environment literals in Rego

Rego policy rules must not contain string literals that represent environment names, namespace names, or any other environment-specific identifiers. Environment-specific behavior is achieved exclusively through data namespace lookups keyed by `input.environment` via the Parameter Resolution Contract (System Design §4).

Concretely: the strings `"prod"`, `"staging"`, `"production"`, `"kube-system"`, or any other infrastructure-specific identifiers must not appear as literal values in Rego rule bodies. They belong in the params data namespace (`evidra/data/params/data.json`) as values within `by_env` maps.

**Enforcement scope:** CI enforces only the explicit, documented subset of infrastructure-specific patterns listed in the CI lint rules (§3). Any additional infrastructure-specific identifier patterns beyond the CI blocklist are subject to code review until explicitly added to the CI pattern set.

Test files are exempt from this invariant — tests may use literal environment names as test fixtures.

### INV-3: All tunables in params namespace

Every numeric limit, string allowlist, string denylist, and categorical control used in policy evaluation must be defined in the bundle's params namespace at `data.evidra.data.params` (sourced from `evidra/data/params/data.json`). No tunable parameter may be defined as:
- A Rego literal (e.g., `count > 5`)
- A Go constant
- An environment variable read at evaluation time
- A default value in a Rego rule head
- An entry under `data.evidra.data.thresholds` (forbidden namespace)
- An entry under `data.evidra.data.environments` (forbidden namespace)

The `data.evidra.data.params` namespace is the single source of truth for all tunable policy parameters. Each param must follow the structure and resolution contract defined in System Design §4.

### INV-4: Single-bundle execution only

The engine accepts exactly one bundle artifact per evaluation. The bundle path must be a single filesystem path to a `.tar.gz` file. The engine must reject the following before any bundle loading occurs:
- Multiple `--bundle` flag values
- A directory path as bundle input
- A glob pattern as bundle input
- An environment variable containing multiple paths

There is no mechanism for loading, composing, layering, or merging multiple bundles. The single bundle contains the complete policy universe.

### INV-5: Bundle revision in evidence

Every evidence record produced from a bundle-based evaluation must contain a non-empty `bundle_revision` field that is an exact string match of the `.manifest.revision` from the loaded bundle. The engine MUST populate `bundle_revision` by calling `PolicySource.BundleRevision()`, not by parsing or inspecting `PolicyRef()`. When `BundleRevision()` returns empty string (`LocalFileSource` path for development and testing), `PolicyRef` remains the sole policy identity. In all other cases, `BundleRevision` is authoritative.

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

When `BundleRevision` is non-empty in an evidence record, `BundleRevision` is the sole authoritative policy identity. `PolicyRef` must not be used for audit, replay, provenance, or release lookup in this case. `PolicyRef` is informational only and MUST NOT be used for integrity or provenance decisions when `BundleRevision` is present. For `BundleSource`, `PolicyRef` returns the manifest revision (not a content hash) — content integrity in bundle mode is provided by the release artifact checksum and the build/release pipeline. No evidence consumer, reporting tool, or external integration may treat `PolicyRef` as policy identity when `BundleRevision` is present. See System Design §9 for the complete authority table.

### INV-10: Unknown environment is bundle author responsibility

The engine must not implement fail-open or fail-closed semantics for unknown environments. The engine must not reject evaluations where the environment label is unknown. The engine must not log warnings about unknown environments. The bundle author is solely responsible for defining behavior when `input.environment` does not resolve through any step of the Parameter Resolution Contract (System Design §4). Every bundle must include test coverage for the unknown-environment path.

### INV-11: No semantic branching on PolicyRef() value

**INVARIANT: Engine MUST NOT branch on the semantic meaning of `PolicyRef()` value.**

`PolicyRef()` returns an opaque string. The engine must treat it as a passive identifier only — a value to be stored, forwarded, and recorded, never inspected. The engine must not inspect, parse, prefix-check, or pattern-match the value. The engine must not branch on whether the value looks like a SHA-256 hash, a manifest revision, or any other recognizable format.

The following conditional patterns are prohibited on `PolicyRef()` values:
- `strings.HasPrefix` / `strings.HasSuffix` / `strings.Contains`
- Regular expression matching
- Length-based inference (e.g., `len(ref) == 64` to detect a hash)
- Format detection of any kind

Policy identity authority is determined exclusively by `PolicySource.BundleRevision()` return value (non-empty vs empty), not by the format or content of `PolicyRef()`. See System Design §7 for the full invariant statement and §9 for the policy identity authority table.

Any branching on `PolicyRef()` value is release-blocking.

### INV-12: Forbidden data namespaces

The following data namespaces are forbidden and must not appear in any bundle artifact, Rego rule, or data file:
- `data.evidra.data.thresholds`
- `data.evidra.data.environments`

All tunable configuration must reside under `data.evidra.data.params` following the param structure and Parameter Resolution Contract defined in System Design §4. References to the forbidden namespaces in Rego, JSON data files, or Go source code are release-blocking violations.

### INV-13: Param keys must not encode context

Param keys within `data.evidra.data.params` must not encode environment names, severity levels, ordinal numbers, or version numbers. Param keys describe the configuration dimension only. Environment-specific values are expressed through `by_env` maps within each param entry, not through the param key itself. See System Design §4 for the param identity requirements.

### INV-14: No type assertions on PolicySource for metadata

The engine MUST NOT use Go type assertions, type switches, or interface-satisfaction checks on `PolicySource` to determine whether bundle metadata is available. The engine MUST use only the `PolicySource` interface methods (`BundleRevision()`, `ProfileName()`) to obtain metadata. These methods return empty strings for non-bundle sources, which the engine records as-is. No conditional logic based on the concrete type of `PolicySource` is permitted.

### INV-15: OPA-compatible data file naming

All data files in the bundle MUST be named `data.json` (or `data.yaml`) and placed in subdirectories that map to the desired OPA data namespace. Files with other names (e.g., `params.json`, `rule_hints.json`, `thresholds.json`, `environments.json`) MUST NOT appear under `evidra/data/` — OPA will silently ignore them, causing data to be absent at runtime with no error.

The required data file layout is:
- `evidra/data/params/data.json` — maps to `data.evidra.data.params`
- `evidra/data/rule_hints/data.json` — maps to `data.evidra.data.rule_hints`

---

## 2. Prohibited Patterns

Each pattern below is an anti-pattern that violates one or more architectural invariants. Code reviews and automated checks must reject these patterns.

### ANTI-1: Hardcoded thresholds

**What it looks like:**
- Rego: `count > 5` instead of resolving the value from `data.evidra.data.params[param_key]` via the Parameter Resolution Contract
- Go: `const maxDeleteCount = 5` used in evaluation logic
- Rego: `default mass_delete_max = 5` as a rule default

**Why it is prohibited:** Hardcoded thresholds bypass the params namespace, making them invisible to bundle configuration and impossible to vary by environment without code changes. They violate INV-3.

**Correct alternative:** All thresholds defined as param entries in `evidra/data/params/data.json` under `data.evidra.data.params`, accessed via the Parameter Resolution Contract (System Design §4).

### ANTI-2: Inline environment checks

**What it looks like:**
- Rego: `input.environment == "prod"` or `action_namespace(action) == "kube-system"`
- Go: `if env == "prod" { ... }`
- Rego: `default is_production = false` with `is_production { input.environment == "production" }`

**Why it is prohibited:** Inline environment checks create an implicit enumeration of known environments. Adding a new environment requires modifying Rego or Go code rather than adding a `by_env` entry. They violate INV-1 and INV-2.

**Correct alternative:** Rego rules look up environment-specific configuration from `data.evidra.data.params` using `input.environment` as the `by_env` key per the Parameter Resolution Contract.

### ANTI-3: Silent fallback bundles

**What it looks like:**
- Loading a "default" bundle when the specified bundle is not found
- Automatically falling back to `LocalFileSource` when bundle loading fails
- Using a bundled-in "base policy" that activates when no bundle is provided
- Applying a "minimum policy" when the manifest is invalid
- Trying a secondary bundle path when the primary path fails

**Why it is prohibited:** Silent fallbacks mask configuration errors and break determinism. If the specified bundle is invalid or missing, the evaluation must fail with an error. The operator must know that their policy did not load, not receive a decision from an unexpected fallback policy. Violates INV-4 and INV-7.

**Clarification:** Explicitly selecting `LocalFileSource` via `--policy`/`--data` flags for development and testing is not a silent fallback — it is an explicit mode choice. The prohibited pattern is the engine silently switching from bundle mode to file mode when bundle loading fails.

**Correct alternative:** Hard failure with a descriptive error message identifying the specific failure (missing file, invalid manifest, parse error, I/O error).

### ANTI-4: Multi-bundle creep

**What it looks like:**
- Accepting a `--bundle` flag multiple times
- Accepting a directory of bundles
- Implementing a "bundle search path"
- A "base bundle + override bundle" composition model
- Separate bundles for "rules" and "data"

**Why it is prohibited:** Multi-bundle composition introduces merge semantics, precedence rules, conflict resolution, and non-obvious interaction effects. It destroys the property that the single bundle is the complete policy universe. It makes deterministic replay dependent on bundle ordering. Violates INV-4.

**Correct alternative:** All policy and data for a given evaluation context are packaged in a single bundle. If different environments need different values, those values coexist as `by_env` entries within param entries in `data.evidra.data.params`.

### ANTI-5: Engine-level environment defaults

**What it looks like:**
- Go: `if environment == "" { environment = "default" }`
- Go: defaulting to a "safe" environment when none is provided
- Rego: `default environment = "base"`

**Why it is prohibited:** Default environment values create an invisible fallback that produces decisions without the caller's explicit intent. If the caller does not provide an environment, the behavior should be determined by the Parameter Resolution Contract within the policy (which the policy author controls), not by the engine. Violates INV-1.

**Correct alternative:** The engine passes the environment label as-is, including empty string. The Parameter Resolution Contract (System Design §4) determines what happens when the `by_env` key is missing or empty.

### ANTI-6: Data split across bundle and runtime

**What it looks like:**
- Some param values in the bundle's data namespace, others in environment variables
- OPA `data.evidra.data.params` supplemented by Go-injected runtime data
- Evidence metadata fields used as implicit policy inputs

**Why it is prohibited:** Splitting policy data across the bundle and the runtime breaks the single-source-of-truth property. The bundle alone must fully determine policy behavior for a given input and environment. Violates INV-3.

**Correct alternative:** All policy data in the bundle under `data.evidra.data.params`. Runtime provides only the input document and the environment label.

### ANTI-7: Non-deterministic output ordering

**What it looks like:**
- Returning `Hits`, `Hints`, or `Reasons` in OPA set iteration order (non-deterministic)
- Using Go map iteration order to populate decision output arrays
- Sorting only during tests but not during production evaluation

**Why it is prohibited:** Non-deterministic output ordering makes evidence records non-reproducible, breaks byte-identical determinism assertions, and produces flaky tests. Violates INV-8.

**Correct alternative:** The engine must sort all multi-value decision fields lexicographically (ascending UTF-8 byte order) unconditionally after every OPA evaluation, before returning the decision to any caller.

### ANTI-8: Use of forbidden data namespaces

**What it looks like:**
- Rego: `data.evidra.data.thresholds[env].mass_delete_max`
- Rego: `data.evidra.data.environments[env].protected_namespaces`
- JSON data file named `thresholds.json` or `environments.json` under `evidra/data/`
- Go code referencing `thresholds` or `environments` as OPA data namespace paths

**Why it is prohibited:** The `thresholds` and `environments` namespaces are superseded by the unified `data.evidra.data.params` namespace. Maintaining separate namespaces fragments tunable configuration, prevents consistent application of the Parameter Resolution Contract, and creates ambiguity about which namespace is authoritative. Violates INV-12.

**Correct alternative:** All tunable configuration in `data.evidra.data.params` (sourced from `evidra/data/params/data.json`) with `by_env` maps for environment-specific values, following the Parameter Resolution Contract (System Design §4).

### ANTI-9: Direct map access bypassing Parameter Resolution Contract

**What it looks like:**
- Rego: accessing `data.evidra.data.params[key].by_env[input.environment]` directly without falling through to `by_env["default"]` or `safety_fallback`
- Rego: accessing a param value without handling the undefined case
- Rego: using `object.get` with an ad-hoc default instead of following the resolution chain

**Why it is prohibited:** Direct access without following the full resolution chain produces inconsistent fail-open/fail-closed behavior across rules. Every rule that consumes a param value must resolve it through the same deterministic sequence. Violates INV-10 and the Parameter Resolution Contract (System Design §4).

**Correct alternative:** Use the shared Rego helper `resolve_param` (located in `evidra/policy/defaults.rego`, package `evidra.policy`) that implements the Parameter Resolution Contract: environment-specific lookup, then default, then safety_fallback, with explicitly documented behavior for the unresolved case. All Rego rules that consume tunable parameters MUST call `resolve_param` rather than implementing the resolution chain inline.

### ANTI-10: Incorrectly named data files in bundle

**What it looks like:**
- `evidra/data/params.json` instead of `evidra/data/params/data.json`
- `evidra/data/rule_hints.json` instead of `evidra/data/rule_hints/data.json`
- Any `.json` file under `evidra/data/` that is not named `data.json`

**Why it is prohibited:** OPA bundles only load data files named `data.json` or `data.yaml`. Files with other names are silently ignored. Data intended for `data.evidra.data.params` will be absent at runtime with no error, causing all param lookups to return `undefined` and all rules to fail open silently. Violates INV-15.

**Correct alternative:** Place data files as `data.json` inside subdirectories: `evidra/data/params/data.json`, `evidra/data/rule_hints/data.json`.

### ANTI-11: Type assertions on PolicySource for metadata access

**What it looks like:**
- Go: `if bs, ok := src.(*BundleSource); ok { rev = bs.Revision }`
- Go: `switch src.(type) { case *BundleSource: ... case *LocalFileSource: ... }`
- Go: using `reflect` to inspect the concrete type of `PolicySource`

**Why it is prohibited:** Type assertions couple the engine to concrete `PolicySource` implementations, introduce implicit source-type branching, and bypass the interface contract. The `PolicySource` interface exposes `BundleRevision()` and `ProfileName()` explicitly — these MUST be the only mechanism for obtaining metadata. Violates INV-14.

**Correct alternative:** Call `PolicySource.BundleRevision()` and `PolicySource.ProfileName()` directly. For `LocalFileSource`, these return empty strings. The engine records whatever is returned without conditional logic.

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
| Data namespace validation | Directory structure check | All JSON data files are under `evidra/data/` as `data.json` in subdirectories | Yes |
| No imports from forbidden packages | `go vet` / import analysis | Dependency boundary checks per invariants | Yes |
| No forbidden data namespace references | `grep` / AST scan | All `.rego` and `.go` files | Yes |
| Params structure validation | JSON schema check | `evidra/data/params/data.json` | Yes |
| No type assertions on PolicySource | AST scan | All `.go` files in `pkg/`, `cmd/` | Yes |
| Data files correctly named | File name check | No `.json` files under `evidra/data/` except `data.json` in subdirectories | Yes |

### Rego lint patterns to detect

The CI pipeline must scan non-test `.rego` files for these patterns and reject matches. This pattern set is the enforced subset of INV-2; infrastructure-specific identifiers not listed here are subject to code review until explicitly added:

| Pattern | Regex (approximate) | Violation |
|---|---|---|
| Environment literal comparison | `==\s*"(prod\|staging\|production\|dev\|test)"` | INV-2 |
| Namespace literal in deny/warn | `==\s*"(kube-system\|default\|kube-public)"` in rule bodies | INV-2 |
| Numeric literal in comparison | `[><=]+\s*\d+` in deny/warn rule bodies (not in test files) | INV-3 |
| Default threshold value | `default\s+\w+\s*=\s*\d+` | INV-3 |
| Reference to forbidden `thresholds` namespace | `data\.evidra\.data\.thresholds` | INV-12 |
| Reference to forbidden `environments` namespace | `data\.evidra\.data\.environments` | INV-12 |
| Direct `by_env` access without `resolve_param` helper | `\.by_env\[` in rule bodies outside `defaults.rego` | INV-10 (advisory, promoted to blocking when AST checks are available) |

**Static analysis method preference:** Regex-based pattern matching is a baseline detection mechanism suitable for initial CI implementation. It is not sufficient as the sole long-term enforcement strategy due to inherent false-positive and false-negative risks (e.g., regex cannot distinguish a string literal in a comment from one in a rule body).

Where feasible, CI checks must operate on parsed ASTs rather than raw text:
- **Rego:** Use `opa parse --format json` to obtain the Rego AST. Detect string literals in rule bodies by inspecting term nodes. Detect references to forbidden data namespaces by inspecting ref nodes. This eliminates false positives from comments and string constants used in non-comparison contexts.
- **Go:** Use `go/ast` or `go/analysis` framework to detect conditional expressions referencing the environment variable, references to forbidden namespace paths, and type assertions on `PolicySource`. This eliminates false positives from comments, documentation strings, and unrelated variables.

AST-based checks are the preferred enforcement mechanism. Regex checks are the mandatory fallback and remain release-blocking until AST-based equivalents are in place.

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
| References to `thresholds` or `environments` as OPA data namespace paths in Go strings | Grep for `"thresholds"` or `"environments"` in data path construction | INV-12 |
| Type assertions on `PolicySource` (`src.(*BundleSource)`, `src.(type)`) | AST scan for type assertions on PolicySource interface | INV-14 |

### Params structure validation

CI must validate that `evidra/data/params/data.json` conforms to the required param structure:

| Check | Requirement | Blocks merge |
|---|---|---|
| File is named `data.json` and located at `evidra/data/params/data.json` | Correct path for OPA namespace mapping | Yes |
| JSON root is a flat object (no wrapper key) | Root keys are param keys, not a `"params"` wrapper | Yes |
| Every param entry has `by_env` | Each param key maps to an object containing at least `by_env` | Yes |
| `by_env` is an object | `by_env` is a JSON object (map), not an array or scalar | Yes |
| `by_env` has at least one entry | Empty `by_env` maps are rejected | Yes |
| `safety_fallback` type matches `by_env` value types | If present, `safety_fallback` must be the same JSON type as the `by_env` values | Advisory |
| `unresolved_behavior` present when needed | If neither `by_env["default"]` nor `safety_fallback` is present, `unresolved_behavior` must be `"fail_open"` or `"fail_closed"` | Yes |
| No param key contains environment names as standalone dot-segments | Split param key on `.`; no segment matches `prod`, `staging`, `dev`, `test` as a whole segment | Yes |
| No param key contains ordinal suffixes | Param keys must not match `.*-\d+$` or `.*_\d+$` | Yes |

### Review checklist

Every pull request that modifies policy, engine, or evidence code must be reviewed against this checklist:

1. Does the change preserve single-bundle execution? (No multi-bundle loading introduced)
2. Are all tunable values sourced from `data.evidra.data.params`? (No new thresholds in Rego or Go, no references to forbidden namespaces)
3. Is the environment label still opaque? (No new Go branching or Rego literals based on environment)
4. Does the evidence schema remain sufficient for deterministic replay? (bundle_revision, profile_name, environment_label, input_hash all present)
5. Does the bundle manifest remain valid? (revision non-empty, roots include "evidra")
6. Are dependency boundaries preserved? (No forbidden imports introduced)
7. If a new rule is added, does it follow the data-driven pattern? (Lookup-based via Parameter Resolution Contract, not literal-based)
8. If a new param is added, does it follow the param structure? (`by_env` map, documented unresolved behavior via `unresolved_behavior` field)
9. Are there any references to `data.evidra.data.thresholds` or `data.evidra.data.environments`? (Must be zero)
10. Are all data files named `data.json` in appropriate subdirectories? (No `params.json`, `rule_hints.json`, etc.)
11. Does evidence binding use `PolicySource.BundleRevision()` and `PolicySource.ProfileName()`? (No type assertions, no PolicyRef parsing)

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
| All data files are `data.json` in subdirectories under `evidra/data/` | No data files under `evidra/policy/`; no non-`data.json` files under `evidra/data/` |
| All Rego `package` declarations start with `evidra.policy` | Parse each `.rego` file and check package path |
| No files outside `evidra/` and `.manifest` | Archive does not contain unexpected paths |
| No `thresholds.json` or `environments.json` anywhere in the bundle | Forbidden data files must not exist | Yes |
| No `params.json` or `rule_hints.json` directly under `evidra/data/` | Incorrectly named files must not exist (OPA would ignore them) | Yes |

---

## 4. Drift Detection Model

Drift is the gradual, accidental violation of architectural invariants through incremental changes that individually seem harmless but collectively erode the architecture.

### Detection signals

| Signal | Invariant at risk | Detection method |
|---|---|---|
| New Go conditional on environment value | INV-1 | Grep/AST scan for `environment` in conditional expressions |
| New string literal in Rego rule body matching infrastructure names | INV-2 | Grep for known infrastructure patterns (`prod`, `staging`, `kube-*`) |
| New numeric literal in Rego deny/warn rule body | INV-3 | Grep for `[><=]\s*\d+` in rule files |
| Evidence record with empty `bundle_revision` in production | INV-5 | Query evidence store for records where `bundle_revision` is empty |
| Published artifact whose manifest revision does not match filename | INV-6 | Release pipeline integrity check |
| New import from a forbidden package | Dependency boundaries | Import graph analysis via `go list -json ./...` |
| New `--bundle` flag accepting multiple values or a directory | INV-4 | Code review + flag definition analysis |
| New Rego `default` statement setting a numeric value | INV-3 | Grep for `default\s+\w+\s*=\s*\d` |
| Bundle artifact checksum mismatch on rebuild | Determinism | Rebuild from tag, compare checksums |
| Decision output arrays not sorted | INV-8 | Determinism test with multi-rule input, verify sorted output |
| Unsorted slice assignment to decision fields in Go | INV-8 | Code review + grep for direct OPA result-to-slice assignment without sort |
| Conditional logic referencing `PolicyRef` return value | INV-11 | Grep/AST scan for `PolicyRef` in conditional expressions, `strings.Has*`/regex/length checks on PolicyRef values |
| Reference to `data.evidra.data.thresholds` or `data.evidra.data.environments` in any source file | INV-12 | Grep scan across all `.rego`, `.go`, and `.json` files |
| Param key containing environment name, severity, or ordinal | INV-13 | Grep/JSON scan of `evidra/data/params/data.json` keys |
| Type assertion on `PolicySource` in engine code | INV-14 | AST scan for type assertions on PolicySource |
| Direct `by_env` access in rule files without `resolve_param` | INV-10 | Grep for `.by_env[` in rule files (not in `defaults.rego`) |
| Existence of `thresholds.json` or `environments.json` in bundle artifact | INV-12 | Archive content listing check |
| Data file not named `data.json` under `evidra/data/` | INV-15 | Archive content listing: verify all `.json` files under `evidra/data/` are named `data.json` in subdirectories |

### Detection cadence

| Cadence | Trigger mechanism | Checks performed |
|---|---|---|
| **Every pull request** | CI pipeline (automated, mandatory) | All CI lint rules: Rego syntax, Go patterns, manifest validation, namespace validation, import graph analysis, forbidden namespace references, params structure validation, data file naming, type assertion scan |
| **Every release** | Release pipeline (automated, mandatory) | Bundle integrity (manifest-filename-tag consistency), deterministic rebuild verification, smoke test evaluation, checksum generation, forbidden file absence in archive, data file naming validation |
| **Weekly** | Scheduled CI job (cron, automated) | Full evidence store scan for records missing `bundle_revision`. Full Go AST scan for environment branching and type assertions. Full Rego scan for literal patterns and forbidden namespace references. Expired exception scan. Params key audit. |
| **Quarterly** | Calendar-triggered review (manual, documented) | Dependency graph visualization and review. Params namespace coverage audit. Rule-to-data complexity ratio analysis. Results recorded in `ai/AI_DECISIONS.md`. |

Weekly and quarterly scans must produce machine-readable reports. Weekly scan failures must create tracked issues automatically. Quarterly review findings must be documented before the next release.

### Remediation policy

| Severity | Condition | Response |
|---|---|---|
| **Blocking** | Any invariant violation detected in CI | PR cannot merge until resolved |
| **Blocking** | Release artifact fails integrity checks | Release is not published |
| **Blocking** | Reference to forbidden data namespace detected | PR cannot merge until resolved |
| **Blocking** | Incorrectly named data file in bundle | PR cannot merge until resolved |
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
- **INV-12** (forbidden data namespaces)
- **INV-14** (no type assertions on PolicySource)
- **INV-15** (OPA-compatible data file naming)

Expired exceptions without resolution are treated as blocking violations. The weekly scheduled scan must detect expired exceptions and create blocking issues automatically. An expired exception blocks all releases until resolved.
