# System and Software Architecture: OPA Bundles as Primary Policy Artifact

**Date:** 2026-02-24
**Author:** Claude Opus 4.6 (Principal Architect role)
**Status:** Draft (Refined 2026-02-24)
**Scope:** System architecture for adopting OPA Bundle as the sole policy artifact format in Evidra

---

## 1. Architectural Overview

### Current State

Evidra loads policy from loose Rego files and a standalone `data.json` via `pkg/policysource.LocalFileSource`. The `LoadPolicy` method walks a directory tree collecting `.rego` files into a `map[string][]byte`. The `LoadData` method reads a single JSON file as raw bytes. Both are passed to `pkg/policy.NewOPAEngine`, which compiles them into a prepared OPA query against `data.evidra.policy.decision`.

Policy identity is derived from a content-addressed SHA-256 hash of all module names and contents, computed by `LocalFileSource.PolicyRef()`. This hash is stamped onto every `policy.Decision` by `pkg/runtime.Evaluator` and recorded in the `EvidenceRecord.PolicyRef` field.

The current policy profile (`policy/profiles/ops-v0.1/`) stores thresholds in `data.json` under the `thresholds` namespace and rule hints under `rule_hints`. Six Rego rules reference `data.thresholds.mass_delete_max` for the sole numeric threshold. Environment is not modeled — rules match namespace strings directly (e.g., `"prod"`, `"kube-system"`) as inline literals.

Key limitations:
- No standardized packaging format. Policy is a directory of files, not a versioned artifact.
- `PolicyRef` is a content hash, not a release revision. Two different builds with identical content produce the same ref, but there is no way to trace a ref back to a release without external metadata.
- Environment-specific configuration requires separate data files or inline Rego literals. The current profile hardcodes namespace strings.
- No manifest. The set of files that constitute "the policy" is implicitly defined by directory traversal.

### Target State

OPA Bundle becomes the sole policy artifact. A bundle is a `.tar.gz` archive conforming to the OPA Bundle specification, containing a `.manifest` file, Rego modules, and JSON data documents under declared namespace roots.

Every evaluation loads exactly one bundle. The `.manifest.revision` field replaces `PolicyRef` as the authoritative policy identity in evidence records. Environment is an opaque string label resolved entirely within the bundle's data namespace — no Go-side branching, no Rego-side environment literals.

### Component Diagram

```
                           ┌───────────────────────────┐
                           │           CLI             │
                           │  cmd/evidra, cmd/evidra-mcp│
                           │                           │
                           │  Inputs:                  │
                           │   --bundle <path.tar.gz>  │
                           │   --environment <label>   │
                           │   --profile <name>        │
                           └────────────┬──────────────┘
                                        │
                                        ▼
                           ┌───────────────────────────┐
                           │         Engine            │
                           │    pkg/validate           │
                           │                           │
                           │  Orchestrates:            │
                           │   load → eval → record    │
                           └──────┬─────────┬──────────┘
                                  │         │
                        ┌─────────▼───┐     │
                        │Bundle Loader│     │
                        │(new package)│     │
                        │             │     │
                        │ Reads .tar.gz    │
                        │ Validates .manifest
                        │ Extracts modules │
                        │ Extracts data    │
                        │ Returns artifact │
                        └─────────┬───┘     │
                                  │         │
                                  ▼         │
                        ┌─────────────────┐ │
                        │ OPA Evaluation  │ │
                        │  pkg/policy     │ │
                        │  pkg/runtime    │ │
                        │                 │ │
                        │ Receives:       │ │
                        │  modules + data │ │
                        │  from artifact  │ │
                        │ Returns:        │ │
                        │  Decision       │ │
                        └─────────┬───────┘ │
                                  │         │
                                  ▼         ▼
                        ┌───────────────────────────┐
                        │        Evidence           │
                        │     pkg/evidence          │
                        │                           │
                        │  Records:                 │
                        │   bundle_revision         │
                        │   profile_name            │
                        │   environment_label       │
                        │   input_hash              │
                        │   decision fields         │
                        └───────────────────────────┘
```

Data flows strictly downward. No layer depends on the layer above it.

---

## 2. Strategic Decision: OPA Bundle as Sole Policy Artifact

### Why no custom format

Evidra's current approach — loading loose files from a directory — is functionally equivalent to an unpacked OPA bundle without a manifest. The OPA Bundle specification already defines:
- A `.manifest` file with `revision` and `roots` fields
- A standard archive layout for Rego modules and JSON data
- Namespace isolation via `roots` declarations
- A well-defined loading contract supported by the OPA Go SDK

Introducing a custom packaging format would duplicate all of this while adding maintenance burden and breaking compatibility with OPA ecosystem tooling (`opa build`, `opa run --bundle`).

### Why bundle revision is authoritative

The current `PolicyRef` (SHA-256 of module contents) is a content hash. It answers "what bytes were loaded?" but not "what release produced them?" Two builds from different Git commits could produce the same hash if content is identical. The hash also does not encode data — changing `data.json` thresholds does not change `PolicyRef` because `LocalFileSource.PolicyRef()` hashes only `.rego` modules, not data files.

The bundle `.manifest.revision` is an explicit, human-assigned identity tied to a release process. It answers "which specific, named policy release produced this decision?" Combined with the `input_hash`, it provides full deterministic replay capability: given the revision, the exact bundle artifact can be retrieved; given the input hash, the exact input can be verified.

---

## 3. Bundle-Based Policy Architecture

### Required Directory Structure

A valid Evidra policy bundle must conform to this internal layout:

```
.manifest                                  # Required: revision, roots
evidra/
  policy/
    decision.rego                          # Aggregator: computes data.evidra.policy.decision
    defaults.rego                          # Shared helper rules
    rules/
      deny_kube_system.rego               # One file per deny/warn rule
      deny_prod_without_approval.rego
      deny_public_exposure.rego
      deny_mass_delete.rego
      warn_breakglass.rego
      warn_autonomous_execution.rego
  data/
    thresholds.json                        # Numeric limits, keyed by environment
    rule_hints.json                        # Remediation hints, keyed by rule ID
    environments.json                      # Per-environment configuration overrides
```

The top-level `evidra/` directory corresponds to the OPA data namespace `data.evidra`. All Rego modules use packages under `evidra.policy`. All data documents are accessible under `data.evidra.data`.

The shim file (`policy.rego` in the current profile) is eliminated. The bundle manifest declares `roots: ["evidra"]`, which gives the bundle exclusive ownership of the entire `data.evidra` namespace. The decision query remains `data.evidra.policy.decision`.

### Manifest Requirements

The `.manifest` file must contain:

| Field | Requirement |
|---|---|
| `revision` | Mandatory. Non-empty string. Immutable once released. Must match the Git tag and artifact filename. |
| `roots` | Mandatory. Must declare exactly `["evidra"]` for Evidra policy bundles. |
| `metadata.profile_name` | Mandatory. Non-empty string. Stable across revisions of the same policy family. Sole source of `ProfileName` in `BundleArtifact`. |

Example `.manifest`:
```json
{
  "revision": "ops-v0.1.0-a3f8c91",
  "roots": ["evidra"],
  "metadata": {
    "profile_name": "ops-v0.1"
  }
}
```

The bundle loader must reject any bundle where `revision` is absent, empty, or where `roots` does not include `"evidra"`.

### Data Namespace Isolation

All policy-configurable values must reside in the data namespace, never in Rego literals or Go constants.

| Current location | Target location (data namespace) |
|---|---|
| `data.thresholds.mass_delete_max` (flat) | `data.evidra.data.thresholds[env_label].mass_delete_max` |
| `data.rule_hints` (flat) | `data.evidra.data.rule_hints` (unchanged, not env-specific) |
| Inline `"prod"` in Rego | `data.evidra.data.environments[env_label].protected_namespaces` |
| Inline `"kube-system"` in Rego | `data.evidra.data.environments[env_label].restricted_namespaces` |

The data namespace is the sole authority for all configurable policy parameters. Rego rules perform lookups into this namespace using the environment label. Go code never inspects or modifies data namespace contents.

---

## 4. Data-Driven Environment Model

### Environment as opaque string

The environment label is a caller-supplied string (e.g., `"prod"`, `"staging"`, `"eu-west-1-prod"`, `"test-local"`). It is:
- Passed as part of the OPA input document under `input.environment`
- Forwarded unchanged by the Go engine — no parsing, no validation, no mapping
- Used by Rego rules solely as a lookup key into the data namespace

### No fixed environment enum

The Go codebase must not define an enumeration of valid environments. There is no `const EnvProd = "prod"`. There is no `switch` or `if` statement that inspects the environment value. The engine is environment-agnostic.

### No environment literals in Rego

Rego rules must not contain string literals that represent environment names. Instead of:

```
deny["POL-PROD-01"] if { action_namespace(action) == "prod" }
```

The rule becomes:

```
deny["POL-PROD-01"] if {
    ns := action_namespace(action)
    env := input.environment
    protected := data.evidra.data.environments[env].protected_namespaces
    ns == protected[_]
    not has_tag(action, "change-approved")
}
```

The set of protected namespaces is defined per environment in the data namespace. Adding a new environment requires only adding a new key to `environments.json` — no Rego or Go changes.

### No environment branching in Go

The Go runtime treats the environment label as an opaque `string` field. It appears in:
1. The input document passed to OPA (set by the caller, forwarded by the engine)
2. The evidence record (recorded as `environment_label`)

It does not appear in any conditional logic, configuration selection, or error handling in Go.

### Deterministic resolution model

Resolution follows a strict, non-negotiable sequence:

1. Caller provides an environment label as input context.
2. Engine places the label into the OPA input document at `input.environment`. The engine must not modify, validate, normalize, or default the value. An empty string is forwarded as an empty string. A missing environment field is forwarded as absent.
3. OPA evaluates the decision query. Rego rules use `input.environment` as a key into `data.evidra.data.environments[input.environment]` to retrieve environment-specific configuration.
4. If the environment key does not exist in the data namespace, the Rego rule's data lookup yields `undefined` per standard OPA semantics. A rule body that references undefined data does not fire. For deny rules, this means the deny condition is not met — the rule does not contribute to the deny set. This is a deterministic, well-defined OPA behavior, not an engine-level decision.
5. The bundle author is solely responsible for fail-open vs fail-closed semantics. To enforce fail-closed behavior for unknown environments, the bundle must include a catch-all deny rule that fires when `data.evidra.data.environments[input.environment]` is undefined. The engine must not implement fail-closed logic. The engine must not inject, substitute, or infer environment values.

The engine never selects, filters, or defaults the environment. The engine never injects a default environment value when none is provided. The engine never falls back to a different bundle or a different data namespace when the environment is unknown.

### Unknown environment contract

**INVARIANT:** Every bundle MUST explicitly define behavior for unknown environments. The absence of such a definition is a policy design decision made by the bundle author — not an engine decision, not a runtime default, and not an ambiguity to be resolved at deployment time.

The contract between engine and bundle for unknown environments is:

| Condition | OPA behavior | Resulting semantics |
|---|---|---|
| `input.environment` is a key in `data.evidra.data.environments` | Data lookup succeeds; rules using that data evaluate normally | Environment is known; policy applies as authored |
| `input.environment` is not a key in `data.evidra.data.environments` | Data lookup yields `undefined`; rule bodies referencing that data do not satisfy | Deny rules that depend on environment data do not fire (fail-open per rule) |
| `input.environment` is absent from OPA input | `input.environment` is `undefined`; all lookups keyed by it yield `undefined` | Same as unknown key — rules depending on environment data do not fire |

This behavior is deterministic: the same unknown label produces the same result on every evaluation. It is not a silent failure — it is a well-defined OPA semantic.

**Bundle author responsibility:** To enforce fail-closed semantics for unknown environments, the bundle MUST include a catch-all deny rule. Example pattern (structural description only, not prescriptive Rego):
- A deny rule that fires when `input.environment` is provided but does not match any key in `data.evidra.data.environments`.
- A deny rule that fires when `input.environment` is absent from the input document.

The engine MUST NOT implement either of these checks. The engine MUST NOT reject evaluations where the environment is unknown. The engine MUST NOT log warnings about unknown environments. The policy is the sole authority over environment handling semantics.

**Build-time validation:** The bundle build process MUST verify that the bundle's test suite includes at least one test case exercising the unknown-environment path. This is a build gate, not a runtime check.

---

## 5. Single-Bundle Execution Model

### Exactly one bundle active

Every evaluation context loads exactly one OPA bundle artifact. The bundle path must be a single filesystem path to a `.tar.gz` file. The following inputs are invalid and must be rejected before evaluation begins:

- Multiple `--bundle` flag values
- A directory path (the engine must not scan directories for bundles)
- A glob pattern
- An environment variable containing multiple paths (colon-separated or otherwise)
- A search path or fallback chain of bundle locations

The engine must accept exactly one bundle path. Any ambiguity in bundle identity is a hard failure.

### No composition

There is no mechanism to layer, overlay, merge, or compose multiple bundles. There is no "base bundle + override bundle" pattern. There is no "shared rules bundle + environment data bundle" pattern. If different environments require different thresholds, those thresholds must coexist as environment-keyed entries within the single bundle's data namespace.

### No merge strategy

The engine must not implement any data or rule merging. The contents of the single loaded bundle are the complete policy universe for that evaluation. There is no concept of precedence, priority, or conflict resolution between bundles because there is only one.

---

## 6. Software Layering and Dependency Boundaries

### Layer definitions and package mapping

| Layer | Packages | Responsibility |
|---|---|---|
| **CLI** | `cmd/evidra`, `cmd/evidra-mcp` | Parse arguments, resolve bundle path, invoke engine |
| **Engine** | `pkg/validate` | Orchestrate load → evaluate → record pipeline |
| **Bundle Loader** | `pkg/bundlesource` (new) | Read `.tar.gz`, validate `.manifest`, extract modules and data, return `BundleArtifact` |
| **OPA Evaluation** | `pkg/policy`, `pkg/runtime` | Compile modules + data into OPA engine, evaluate decision query, stamp revision |
| **Evidence** | `pkg/evidence` | Append decision record with bundle metadata |

### Allowed dependency direction

```
CLI
 └──→ Engine (pkg/validate)
       ├──→ Bundle Loader (pkg/bundlesource)
       ├──→ OPA Evaluation (pkg/runtime → pkg/policy)
       └──→ Evidence (pkg/evidence)
```

The Bundle Loader returns a `BundleArtifact` value type to the Engine. The Engine passes extracted modules and data to the OPA Evaluation layer. The Engine passes decision metadata to Evidence.

### Forbidden dependencies

| From | To | Reason |
|---|---|---|
| `pkg/evidence` | `pkg/policy` or `pkg/runtime` | Evidence must not depend on OPA internals. It records opaque metadata fields. |
| `pkg/bundlesource` | `pkg/validate` | The loader must not depend on the orchestrator. It is a pure extraction utility. |
| `pkg/bundlesource` | `pkg/policy` | The loader must not evaluate policy. It validates bundle structure only. |
| `pkg/policy` | `pkg/bundlesource` | The OPA engine accepts modules and data as arguments. It does not know how they were loaded. |
| `cmd/*` | `pkg/bundlesource` | CLI must not parse bundle internals. It passes the path to the engine. |
| Any package | Environment value inspection | No package may branch on the environment label string. |

### Interface boundary: `runtime.PolicySource`

The existing `runtime.PolicySource` interface remains the contract between the engine and policy loading:

```
PolicySource interface {
    LoadPolicy() (map[string][]byte, error)
    LoadData() ([]byte, error)
    PolicyRef() (string, error)
}
```

`pkg/bundlesource` implements this interface:
- `LoadPolicy` returns the extracted Rego modules from the archive.
- `LoadData` returns the merged data documents as a single JSON object.
- `PolicyRef` returns the `.manifest.revision` string.

**Semantic distinction:** For `LocalFileSource`, `PolicyRef()` returns a SHA-256 content hash of module contents. For `BundleSource`, `PolicyRef()` returns the manifest revision string. These are different kinds of identifiers (content-addressed vs release-named). The engine treats the return value as an opaque string stamped onto decisions. The evidence layer distinguishes them via the separate `BundleRevision` field (see §8): when `BundleRevision` is non-empty, it is the authoritative policy identity; `PolicyRef` is secondary and informational.

**INVARIANT: Engine MUST NOT branch on the semantic meaning of `PolicyRef()` value.**

`PolicyRef()` returns an opaque string. The engine must treat it as a passive identifier only — a value to be stored, forwarded, and recorded, never inspected. The engine must not inspect, parse, prefix-check, or pattern-match the `PolicyRef()` return value. Specifically, the engine must not branch on whether the value looks like:
- A SHA-256 hash (e.g., `strings.HasPrefix(ref, "sha256:")` or length-based detection)
- A manifest revision (e.g., regex matching `<profile>-v<semver>-<hash>`)
- Any other recognizable format

The following conditional patterns are prohibited on `PolicyRef()` values:
- `strings.HasPrefix` / `strings.HasSuffix` / `strings.Contains`
- Regular expression matching
- Length-based inference (e.g., `len(ref) == 64` to detect a hash)
- Format detection of any kind

Policy identity authority is determined exclusively by `BundleRevision` presence (non-empty vs empty), not by the format or content of `PolicyRef()`. See §8 for the complete authority table.

`pkg/policysource.LocalFileSource` continues to satisfy the interface for development, testing, and the migration period. The engine must not contain logic that inspects the `PolicyRef` return value to determine which source type is in use.

### New type: `BundleArtifact`

The bundle loader produces a `BundleArtifact` value that carries metadata beyond what `PolicySource` exposes:

| Field | Type | Source |
|---|---|---|
| `Revision` | `string` | `.manifest.revision` |
| `Roots` | `[]string` | `.manifest.roots` |
| `ProfileName` | `string` | Derived from `.manifest.metadata.profile_name` (see derivation rule below) |
| `Modules` | `map[string][]byte` | Extracted `.rego` files |
| `Data` | `[]byte` | Merged JSON data documents |

`BundleArtifact` is immutable after construction. It must have no exported mutating methods and no exported mutable fields. All exported fields must be read-only (unexported fields set during construction, accessed via exported getter methods or exported as value types in a frozen struct).

**ProfileName derivation rule:** `ProfileName` must be derived from exactly one authoritative source: the `.manifest` file. The manifest schema is extended with an optional `metadata` object containing a `profile_name` field:

```json
{
  "revision": "ops-v0.1.0-a3f8c91",
  "roots": ["evidra"],
  "metadata": {
    "profile_name": "ops-v0.1"
  }
}
```

The derivation rule is:
1. If `.manifest.metadata.profile_name` is present and non-empty, use it as `ProfileName`.
2. If `.manifest.metadata.profile_name` is absent or empty, the bundle loader must return an error. `ProfileName` must not be inferred from the artifact filename, the directory name, or any other source.

There is exactly one derivation path. There is no fallback. Filename-based inference is prohibited because it couples the profile identity to deployment-time naming conventions that are outside the bundle's control.

---

## 7. Determinism Model

### Invariant

Given:
- Identical input (same `ToolInvocation` or `Scenario`, byte-for-byte)
- Identical bundle artifact (same `.manifest.revision`, same archive checksum)
- Identical environment label (same string value, including empty)

The evaluation must produce:
- Identical `Decision.Allow` (boolean)
- Identical `Decision.RiskLevel` (string)
- Identical `Decision.Reason` (string)
- Identical `Decision.Hits` (same elements in same order)
- Identical `Decision.Hints` (same elements in same order)
- Identical `Decision.Reasons` (same elements in same order)

This invariant holds across machines, operating systems, Go versions, and wall-clock time. The evaluation context must contain no implicit state: no system clock reads during OPA evaluation, no random values, no network lookups, no environment variable inspection during evaluation.

### Output ordering requirement

OPA rule evaluation produces sets, which have no guaranteed iteration order. The engine must sort all multi-value decision fields before returning them to the caller:

| Field | Sort order |
|---|---|
| `Hits` (rule IDs) | Lexicographic ascending (UTF-8 byte order) |
| `Hints` | Lexicographic ascending (UTF-8 byte order) |
| `Reasons` | Lexicographic ascending (UTF-8 byte order) |

This sorting must occur in the engine after OPA evaluation and before the decision is returned to callers or recorded in evidence. The sort is applied unconditionally — not only when determinism testing is active.

Map iteration in Go is non-deterministic. No decision field may depend on map iteration order. Any intermediate data structure that uses maps must be converted to a sorted representation before it influences output.

### Immutability requirement for BundleArtifact

Once a `BundleArtifact` is constructed from a `.tar.gz` file, its contents are fixed for the lifetime of the evaluation. The artifact must not be reloaded, patched, or supplemented during evaluation. The Rego modules and data documents extracted at load time are the complete and final policy inputs.

Once a revision string is assigned to a released artifact, that revision must never be reused for a different artifact. Re-tagging or re-publishing under the same revision is a release integrity violation. This is unconditional — there is no exception mechanism for revision reuse.

### Canonical JSON serialization requirement

Decision comparison and evidence recording must use canonical JSON serialization. Canonical JSON is defined for this architecture as:

| Property | Requirement |
|---|---|
| Key ordering | All object keys sorted lexicographically (ascending UTF-8 byte order), applied recursively to nested objects |
| Whitespace | No insignificant whitespace (compact encoding: no spaces after `:` or `,`, no newlines) |
| Floating-point | Not applicable — no floating-point fields in the decision schema. If introduced in future, IEEE 754 representation with no trailing zeros must be specified. |
| Unicode | No escaped ASCII characters (UTF-8 output only). Non-ASCII characters are output as literal UTF-8, not `\uXXXX` escapes. |
| Null handling | Null fields are omitted from output (consistent with `omitempty` semantics in the existing `Decision` struct) |
| Map iteration | All maps must be converted to sorted key order before serialization. Go `encoding/json` sorts map keys by default — this behavior must not be overridden. |

This serialization format must be used for:
1. `InputHash` computation (SHA-256 of the canonical JSON of the input document)
2. Evidence record serialization (for hash-chain computation)
3. Determinism test assertions

Deviation from canonical serialization (e.g., non-deterministic key order from a custom marshaler) is a determinism violation and is release-blocking.

### Verification

Determinism is verified by a mandatory test that:
1. Loads a bundle artifact
2. Evaluates a fixed input with a fixed environment label N times (N ≥ 10)
3. Serializes each decision using canonical JSON (as defined above)
4. Asserts all N serialized outputs are byte-identical

This test must pass for every bundle artifact release. Failure is release-blocking.

---

## 8. Evidence Model Extension

### New fields on `EvidenceRecord`

The `evidence.EvidenceRecord` type gains four fields. These fields are mandatory when the evaluation source is a bundle. They are empty (zero-value) when the evaluation source is the legacy `LocalFileSource`, preserving backward compatibility during migration.

| Field | Type | Description |
|---|---|---|
| `BundleRevision` | `string` | Value of `.manifest.revision` from the loaded bundle. Replaces `PolicyRef` as the primary policy identity for bundle-based evaluations. |
| `ProfileName` | `string` | Logical name of the policy profile (e.g., `"ops-v0.1"`). Derived from the bundle artifact, not from filesystem paths. |
| `EnvironmentLabel` | `string` | The opaque environment string provided at invocation time. Recorded verbatim, never interpreted. |
| `InputHash` | `string` | SHA-256 hash of the canonical serialization of the input (`ToolInvocation` or `Scenario`). Enables input replay verification. |

### JSON schema extension

```json
{
  "event_id": "evt-...",
  "timestamp": "...",
  "policy_ref": "sha256:...",
  "bundle_revision": "ops-v0.1.0-a3f8c91",
  "profile_name": "ops-v0.1",
  "environment_label": "prod",
  "input_hash": "sha256:...",
  "actor": {},
  "policy_decision": {},
  "..."
}
```

`PolicyRef` is retained for backward compatibility and continues to be populated (content hash of modules).

### Policy identity authority rule

**INVARIANT:** When `BundleRevision` is non-empty, it is the sole authoritative policy identity. `PolicyRef` is informational only.

The following table defines which field governs each use case when `BundleRevision` is present:

| Use case | Authoritative field | `PolicyRef` role |
|---|---|---|
| Audit trail: "which policy produced this decision?" | `BundleRevision` | MUST NOT be used |
| Replay: "reproduce this decision" | `BundleRevision` + `InputHash` + `EnvironmentLabel` | MUST NOT be used |
| Provenance: "trace decision to release artifact" | `BundleRevision` | MUST NOT be used |
| Release lookup: "retrieve the artifact" | `BundleRevision` → Git tag → GitHub Release | MUST NOT be used |
| Content integrity: "were the bytes tampered with?" | `PolicyRef` (content hash) | Informational verification |

When `BundleRevision` is empty (legacy `LocalFileSource` path), `PolicyRef` remains the sole policy identity. This is permissible only during migration Phases 1 and 2.

No system — engine, evidence consumer, reporting tool, or external integration — may use `PolicyRef` as policy identity when `BundleRevision` is non-empty. This is a non-optional invariant.

### Why binding revision is mandatory

Without `BundleRevision`, an evidence record says "a decision was made" but cannot answer "by which exact policy release?" The `PolicyRef` content hash is necessary but not sufficient — it proves what bytes were loaded but does not identify which release process produced them. `BundleRevision` closes this gap by linking every decision to a named, retrievable, reproducible artifact.

Without `InputHash`, an evidence record cannot be independently verified. Given the bundle artifact (identified by revision) and the input (identified by hash), any party can replay the evaluation and confirm the recorded decision is correct.

Without `EnvironmentLabel`, the replay is incomplete — the same input evaluated against the same bundle in different environments may produce different decisions (because environment-keyed data differs). Recording the label makes the replay fully deterministic.

---

## 9. Migration Strategy

### Phase 1: Bundle source implementation (parallel path)

Introduce `pkg/bundlesource` implementing `runtime.PolicySource`. The engine gains a `--bundle` flag alongside the existing `--policy`/`--data` flags. When `--bundle` is provided, the engine uses `BundleSource`. When `--policy`/`--data` are provided, the engine uses `LocalFileSource`. Providing both is an error.

During this phase:
- The existing profile directory (`policy/profiles/ops-v0.1/`) is restructured to match the bundle internal layout under `evidra/` namespace.
- A build step produces a `.tar.gz` bundle from the restructured directory.
- Integration tests validate that the bundle-loaded evaluation produces identical decisions to the file-loaded evaluation for a fixed test corpus.

### Phase 2: Rego rule migration to data-driven environment model

Current Rego rules that contain inline namespace literals (`"prod"`, `"kube-system"`) are rewritten to perform data namespace lookups keyed by `input.environment`. The `data.json` is split into environment-keyed data documents (`thresholds.json`, `environments.json`).

This phase changes policy behavior: rules that previously matched `"prod"` unconditionally now match only when `input.environment` is provided and the environment's configuration lists the namespace as protected. Test coverage must verify both the new behavior and the migration from old to new.

### Phase 3: Bundle-only runtime path

The `--policy`/`--data` flags are deprecated. The engine defaults to bundle loading. `LocalFileSource` remains available for development and testing but is no longer the production path. Evidence records without `BundleRevision` are flagged as legacy in reporting.

### Phase 4: Legacy removal

`LocalFileSource` is removed from the production binary. The `--policy`/`--data` flags are removed. All evaluations require a bundle. Evidence records without `BundleRevision` are no longer produced.

### Moving hardcoded thresholds to data namespace

The sole hardcoded threshold today is `data.thresholds.mass_delete_max` (value: 5). This is already in the data namespace, not in Rego. Under the bundle architecture, this moves to `data.evidra.data.thresholds[env_label].mass_delete_max`, enabling per-environment thresholds within the same bundle.

The inline namespace strings (`"prod"`, `"kube-system"`) in deny rules are the primary migration target. These become lookups into `data.evidra.data.environments[env_label].protected_namespaces` and `data.evidra.data.environments[env_label].restricted_namespaces`.

---

## 10. Acceptance Criteria

Every criterion is binary (pass/fail). No criterion is advisory. Failure of any criterion is release-blocking.

| # | Criterion | Verification method | Pass condition |
|---|---|---|---|
| 1 | Engine loads a `.tar.gz` OPA bundle and evaluates `data.evidra.policy.decision` | Integration test: load bundle, evaluate known input | Decision fields match expected values exactly |
| 2 | Engine rejects a bundle with missing `.manifest` | Unit test: bundle archive without `.manifest` | Returns error; no OPA evaluation occurs |
| 3 | Engine rejects a bundle with empty or missing `revision` in `.manifest` | Unit test: manifest with `"revision": ""` and with `revision` key absent | Returns error; no OPA evaluation occurs |
| 4 | Engine rejects a bundle with missing or incorrect `roots` | Unit test: manifest with `roots` absent; manifest with `roots: ["other"]` | Returns error; no OPA evaluation occurs |
| 5 | Engine rejects invocation when multiple bundle paths are provided | Unit test: two `--bundle` flag values | Returns error before bundle loading |
| 6 | Engine rejects invocation when both `--bundle` and `--policy` are provided | Unit test: `--bundle` combined with `--policy` or `--data` | Returns error before bundle loading |
| 7 | Engine rejects a directory path as bundle input | Unit test: `--bundle /path/to/directory/` | Returns error; no directory scanning occurs |
| 8 | Zero Go conditionals inspecting the environment label value | CI gate: `grep -rn` for `environment` in conditional expressions across `pkg/`, `cmd/`, `internal/` | Zero matches (exit code 1 from grep) |
| 9 | Zero environment-name string literals in non-test Rego rule files | CI gate: pattern scan of `.rego` files under `evidra/policy/rules/` | Zero matches |
| 10 | Zero numeric literals in comparisons in deny/warn rule bodies | CI gate: pattern scan of `.rego` files under `evidra/policy/rules/` | Zero matches |
| 11 | Evidence record contains `bundle_revision` matching `.manifest.revision` | Integration test: evaluate via bundle, read evidence record | `record.bundle_revision == manifest.revision` (exact string match) |
| 12 | Evidence record contains `profile_name`, `environment_label`, `input_hash` | Integration test: evaluate via bundle with environment label | All three fields are non-empty strings |
| 13 | Determinism: repeated evaluations produce identical output | Determinism test: N ≥ 10 evaluations with identical input, bundle, environment | All N serialized decisions (JSON with sorted keys) are byte-identical |
| 14 | Output ordering: `Hits`, `Hints`, `Reasons` are lexicographically sorted | Unit test: evaluate input that fires multiple rules | All three arrays are in ascending UTF-8 byte order |
| 15 | `BundleArtifact` has no exported mutating methods | Compilation review: exported method set inspection | Zero exported methods that modify struct fields |
| 16 | `pkg/bundlesource` satisfies `runtime.PolicySource` interface | Compile-time interface satisfaction check | Compiles without error |
| 17 | `pkg/bundlesource` has no imports from `pkg/validate`, `pkg/policy`, or `cmd/*` | `go list -json ./pkg/bundlesource` import analysis | Zero forbidden imports |
| 18 | `pkg/evidence` has no imports from `pkg/policy`, `pkg/runtime`, or `pkg/bundlesource` | `go list -json ./pkg/evidence` import analysis | Zero forbidden imports |
| 19 | Engine passes environment label to OPA without modification | Unit test: set `input.environment` to arbitrary string, verify OPA receives identical string | `input.environment` in OPA input == caller-provided value (byte-identical) |
| 20 | Engine does not inject default environment when none is provided | Unit test: omit environment from invocation, verify OPA input has no `environment` key | `input.environment` is absent from OPA input document |
