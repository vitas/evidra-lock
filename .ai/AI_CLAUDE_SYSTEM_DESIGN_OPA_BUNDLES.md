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

The current policy profile (`policy/profiles/ops-v0.1/`) stores all tunable parameters in `data.json` as flat keys. Six Rego rules reference `data.thresholds.mass_delete_max` for the sole numeric threshold. Environment is not modeled — rules match namespace strings directly (e.g., `"prod"`, `"kube-system"`) as inline literals.

Key limitations:
- No standardized packaging format. Policy is a directory of files, not a versioned artifact.
- `PolicyRef` is a content hash, not a release revision. Two different builds with identical content produce the same ref, but there is no way to trace a ref back to a release without external metadata.
- No unified parameter model. Thresholds, environment configuration, and rule hints are stored in unrelated flat namespaces with no resolution contract for environment-specific values.
- No manifest. The set of files that constitute "the policy" is implicitly defined by directory traversal.

### Target State

OPA Bundle becomes the sole policy artifact. A bundle is a `.tar.gz` archive conforming to the OPA Bundle specification, containing a `.manifest` file, Rego modules, and JSON data documents under declared namespace roots.

Every evaluation loads exactly one bundle. The `.manifest.revision` field replaces `PolicyRef` as the authoritative policy identity in evidence records. Environment is an opaque string label resolved entirely within the bundle's unified params namespace — no Go-side branching, no Rego-side environment literals. All tunable configuration lives under `data.evidra.data.params`.

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

### CLI `--profile` flag semantics

The `--profile` flag selects a bundle artifact by name. It is a CLI-level convenience for resolving the bundle path (e.g., selecting from `policy/bundles/<profile>/`). It does not override or replace any value inside the bundle.

After the bundle is loaded, the engine MUST compare the CLI-provided `--profile` value to `.manifest.metadata.profile_name`. If they do not match exactly (case-sensitive string comparison), the engine MUST reject the invocation with an error before any policy evaluation occurs. This prevents silent misidentification in evidence records.

The manifest `metadata.profile_name` remains the sole authoritative source of `ProfileName` in `BundleArtifact` and in evidence records. The `--profile` flag is a selection and validation mechanism, not an identity source and not an override.

If `--bundle` is provided with an explicit path, `--profile` is optional. If both are provided, the profile-name validation still applies. If neither `--bundle` nor `--profile` is provided and the engine cannot resolve a bundle path, the invocation fails.

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

The current `PolicyRef` (SHA-256 of module contents) is a content hash. It answers "what bytes were loaded?" but not "what release produced them?" Two builds from different Git commits could produce the same hash if content is identical. The hash also does not encode data — changing `data.json` does not change `PolicyRef` because `LocalFileSource.PolicyRef()` hashes only `.rego` modules, not data files.

The bundle `.manifest.revision` is an explicit, human-assigned identity tied to a release process. It answers "which specific, named policy release produced this decision?" Combined with the `input_hash`, it provides full deterministic replay capability: given the revision, the exact bundle artifact can be retrieved; given the input hash, the exact input can be verified.

---

## 3. Bundle-Based Policy Architecture

### Required Directory Structure

A valid Evidra policy bundle must conform to this internal layout:

```
.manifest                                  # Required: revision, roots, metadata.profile_name
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
    params/
      data.json                            # Unified tunable parameters
    rule_hints/
      data.json                            # Remediation hints, keyed by rule ID
```

**OPA data loading requirement:** OPA bundles only load data files named `data.json` or `data.yaml`. Files with other names are silently ignored. All data documents in this bundle MUST be named `data.json` and placed in subdirectories that map to the desired OPA data namespace. The directory path determines the namespace:
- `evidra/data/params/data.json` maps to `data.evidra.data.params`
- `evidra/data/rule_hints/data.json` maps to `data.evidra.data.rule_hints`

The top-level `evidra/` directory corresponds to the OPA data namespace `data.evidra`. All Rego modules use packages under `evidra.policy`. All data documents are accessible under `data.evidra.data`.

The shim file (`policy.rego` in the current profile) is eliminated. The bundle manifest declares `roots: ["evidra"]`, which gives the bundle exclusive ownership of the entire `data.evidra` namespace. The decision query remains `data.evidra.policy.decision`.

**Eliminated files:** The following data files are not part of the bundle layout and must not appear in any bundle artifact:
- `thresholds.json` — replaced by entries in `evidra/data/params/data.json`
- `environments.json` — replaced by `by_env` maps within `evidra/data/params/data.json`
- Any file named `params.json` or `rule_hints.json` directly under `evidra/data/` (OPA will not load these)

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

All policy-configurable values must reside in the data namespace under `data.evidra.data.params`, never in Rego literals or Go constants.

| Current location | Target location (data namespace) | Bundle file path |
|---|---|---|
| `data.thresholds.mass_delete_max` (flat) | `data.evidra.data.params["ops.mass_delete.max_deletes"].by_env[env_label]` | `evidra/data/params/data.json` |
| Inline `"prod"` in Rego | `data.evidra.data.params["k8s.namespaces.protected"].by_env[env_label]` | `evidra/data/params/data.json` |
| Inline `"kube-system"` in Rego | `data.evidra.data.params["k8s.namespaces.restricted"].by_env[env_label]` | `evidra/data/params/data.json` |
| `data.rule_hints` (flat) | `data.evidra.data.rule_hints` (unchanged, not a tunable parameter) | `evidra/data/rule_hints/data.json` |

The `data.evidra.data.params` namespace is the sole authority for all tunable policy parameters. The `data.evidra.data.thresholds` and `data.evidra.data.environments` namespaces are forbidden. Rego rules perform lookups into `data.evidra.data.params` using param keys and the environment label. Go code never inspects or modifies data namespace contents.

### Data file content structure

The JSON root of `evidra/data/params/data.json` MUST be the params map itself — a flat object whose keys are param keys and whose values are param objects. There MUST NOT be a wrapping `"params"` key at the JSON root. OPA's directory-based namespace mapping already places the content at `data.evidra.data.params`; adding a wrapper key would produce `data.evidra.data.params.params`, which is incorrect.

Correct structure of `evidra/data/params/data.json`:
```json
{
  "ops.mass_delete.max_deletes": {
    "by_env": {
      "prod": 5,
      "staging": 20,
      "default": 10
    }
  },
  "k8s.namespaces.restricted": {
    "by_env": {
      "prod": ["kube-system"],
      "staging": ["kube-system"],
      "default": ["kube-system"]
    }
  }
}
```

The JSON root of `evidra/data/rule_hints/data.json` MUST be the rule hints map itself — a flat object whose keys are rule IDs and whose values are arrays of hint strings.

### Data presence validation

The bundle loader MUST validate that required data documents are present and non-empty after OPA bundle loading completes:
- `data.evidra.data.params` MUST exist and be a non-empty object. If absent or empty, the bundle loader MUST return an error before evaluation.
- `data.evidra.data.rule_hints` MUST exist and be a non-empty object. If absent or empty, the bundle loader MUST return an error before evaluation.

This validation catches the case where data files are incorrectly named (and therefore silently ignored by OPA) or are present but contain empty objects.

---

## 4. Unified Params Model

### Param identity

Each tunable value is identified by a stable `param_key` string. Param keys describe the configuration dimension only. They must not encode environment, severity, ordinal numbers, or version numbers.

Param key format: `<domain>.<scope>.<dimension>`

Examples of valid param keys:
- `ops.mass_delete.max_deletes`
- `k8s.namespaces.restricted`
- `k8s.namespaces.protected`
- `argocd.destinations.allowed`
- `ops.public_exposure.approved_sources`

Param keys must be lowercase, dot-separated, and stable once released. Renaming a param key after release is a breaking change that requires a new bundle revision.

### Param structure

Each param entry in `data.evidra.data.params` supports:

| Field | Type | Required | Description |
|---|---|---|---|
| `by_env` | object (map of environment_label to value) | Yes | Environment-specific values. Must contain at least one entry. |
| `by_env["default"]` | any JSON type | No | Fallback value used when the provided environment label is not present in `by_env`. |
| `safety_fallback` | any JSON type | No | Documented hard safety constant used when both environment-specific and default lookups yield no result. Bundle author must explicitly define the behavioral consequence of reaching this fallback. |
| `unresolved_behavior` | string (`"fail_open"` or `"fail_closed"`) | Conditional | Required when neither `by_env["default"]` nor `safety_fallback` is present. Documents whether the rule fails open or fails closed for unlisted environments. Machine-verifiable by the build gate. |

Values within `by_env` and `safety_fallback` may be any standard JSON type: number, boolean, string, array, or object. No custom types. No references to external documents.

### Parameter Resolution Contract

This section defines the normative resolution algorithm for parameter values. All Rego rules that consume tunable parameters MUST follow this contract. The engine MUST NOT implement any part of this resolution — it is entirely within the policy (Rego) domain.

**Canonical helper:** The resolution algorithm MUST be implemented by a shared Rego helper named `resolve_param`, located in `evidra/policy/defaults.rego` (package `evidra.policy`). This helper is responsible for: looking up the environment-specific value from the `by_env` map, falling through to `by_env["default"]`, falling through to `safety_fallback`, and producing the documented unresolved behavior. All Rego rules that consume tunable parameters MUST call `resolve_param` rather than implementing the resolution chain inline.

**Resolution algorithm (strict order):**

1. Obtain `environment_label` from `input.environment`.
2. Lookup `data.evidra.data.params[param_key].by_env[environment_label]`. If the value exists, use it. Resolution is complete.
3. If absent, lookup `data.evidra.data.params[param_key].by_env["default"]`. If the value exists, use it. Resolution is complete.
4. If absent, use `data.evidra.data.params[param_key].safety_fallback`. If the value exists, use it. Resolution is complete.
5. If still unresolved, the behavior MUST be explicitly defined by the bundle author. The bundle author MUST choose one of:
   - Fail-open: the rule does not fire (the Rego rule body does not satisfy due to undefined data, per standard OPA semantics).
   - Explicit deny: a catch-all deny rule fires for the unresolved parameter.

The engine MUST NOT inject default parameter values. The engine MUST NOT interpret parameter semantics. The engine MUST NOT implement any step of this resolution algorithm. Resolution is a Rego-only concern.

**Determinism requirement:** An unknown environment label MUST produce a deterministic outcome. The same unknown label, the same bundle, and the same input MUST always produce the same decision. This is guaranteed by OPA's deterministic handling of undefined data, provided the bundle author follows this contract.

**Bundle author obligation:** Every param entry MUST document its unresolved behavior. If `safety_fallback` is absent and `by_env["default"]` is absent, the param entry MUST include the `unresolved_behavior` field set to either `"fail_open"` or `"fail_closed"`. The bundle's test suite MUST include at least one test case per param exercising the unresolved path.

---

## 5. Data-Driven Environment Model

### Environment as opaque string

The environment label is a caller-supplied string (e.g., `"prod"`, `"staging"`, `"eu-west-1-prod"`, `"test-local"`). It is:
- Passed as part of the OPA input document under `input.environment`
- Forwarded unchanged by the Go engine — no parsing, no validation, no mapping
- Used by Rego rules solely as a lookup key into `data.evidra.data.params[param_key].by_env`

### No fixed environment enum

The Go codebase must not define an enumeration of valid environments. There is no `const EnvProd = "prod"`. There is no `switch` or `if` statement that inspects the environment value. The engine is environment-agnostic.

### No environment literals in Rego

Rego rules must not contain string literals that represent environment names. Instead of hardcoding environment or namespace strings, rules perform lookups into `data.evidra.data.params` using `input.environment` as the `by_env` key. The set of protected namespaces, restricted namespaces, or any other environment-specific configuration is defined entirely in `evidra/data/params/data.json` under the appropriate param key's `by_env` map. Adding a new environment requires only adding a new key to the relevant `by_env` maps — no Rego or Go changes.

### No environment branching in Go

The Go runtime treats the environment label as an opaque `string` field. It appears in:
1. The input document passed to OPA (set by the caller, forwarded by the engine)
2. The evidence record (recorded as `environment_label`)

It does not appear in any conditional logic, configuration selection, or error handling in Go.

### Deterministic resolution model

Resolution follows a strict, non-negotiable sequence:

1. Caller provides an environment label as input context.
2. Engine places the label into the OPA input document at `input.environment`. The engine must not modify, validate, normalize, or default the value. An empty string is forwarded as an empty string. A missing environment field is forwarded as absent.
3. OPA evaluates the decision query. Rego rules use `input.environment` as the `by_env` key within each param entry under `data.evidra.data.params` to retrieve environment-specific configuration, following the Parameter Resolution Contract defined in §4.
4. If the environment key does not exist in any param's `by_env` map, the resolution falls through to `by_env["default"]`, then to `safety_fallback`, then to the bundle author's documented unresolved behavior — all per the Parameter Resolution Contract.
5. The bundle author is solely responsible for fail-open vs fail-closed semantics. To enforce fail-closed behavior for unknown environments, the bundle must include a catch-all deny rule that fires when the environment label does not resolve to any param value. The engine must not implement fail-closed logic. The engine must not inject, substitute, or infer environment values.

The engine never selects, filters, or defaults the environment. The engine never injects a default environment value when none is provided. The engine never falls back to a different bundle or a different data namespace when the environment is unknown.

### Unknown environment contract

**INVARIANT:** Every bundle MUST explicitly define behavior for unknown environments. The absence of such a definition is a policy design decision made by the bundle author — not an engine decision, not a runtime default, and not an ambiguity to be resolved at deployment time.

The contract between engine and bundle for unknown environments is:

| Condition | OPA behavior | Resulting semantics |
|---|---|---|
| `input.environment` resolves via Parameter Resolution Contract (§4) | Param value obtained; rules using that value evaluate normally | Environment is known; policy applies as authored |
| `input.environment` does not resolve via any step of the Parameter Resolution Contract | Data lookup yields `undefined`; rule bodies referencing that data do not satisfy | Deny rules that depend on param data do not fire (fail-open per rule) unless a catch-all deny rule is present |
| `input.environment` is absent from OPA input | `input.environment` is `undefined`; all `by_env` lookups keyed by it yield `undefined` | Same as unknown key — resolution falls through per the Parameter Resolution Contract |

This behavior is deterministic: the same unknown label produces the same result on every evaluation. It is not a silent failure — it is a well-defined OPA semantic combined with the explicit Parameter Resolution Contract.

**Bundle author responsibility:** To enforce fail-closed semantics for unknown environments, the bundle MUST include a catch-all deny rule. The rule fires when `input.environment` is provided but does not resolve to a value through any step of the Parameter Resolution Contract, or when `input.environment` is absent from the input document.

The engine MUST NOT implement either of these checks. The engine MUST NOT reject evaluations where the environment is unknown. The engine MUST NOT log warnings about unknown environments. The policy is the sole authority over environment handling semantics.

**Build-time validation:** The bundle build process MUST verify that the bundle's test suite includes at least one test case exercising the unknown-environment path. This is a build gate, not a runtime check.

---

## 6. Single-Bundle Execution Model

### Exactly one bundle active

Every evaluation context loads exactly one OPA bundle artifact. The bundle path must be a single filesystem path to a `.tar.gz` file. The following inputs are invalid and must be rejected before evaluation begins:

- Multiple `--bundle` flag values
- A directory path (the engine must not scan directories for bundles)
- A glob pattern
- An environment variable containing multiple paths (colon-separated or otherwise)
- A search path or fallback chain of bundle locations

The engine must accept exactly one bundle path. Any ambiguity in bundle identity is a hard failure.

### No composition

There is no mechanism to layer, overlay, merge, or compose multiple bundles. There is no "base bundle + override bundle" pattern. There is no "shared rules bundle + environment data bundle" pattern. If different environments require different values, those values must coexist as `by_env` entries within the single bundle's `data.evidra.data.params` namespace.

### No merge strategy

The engine must not implement any data or rule merging. The contents of the single loaded bundle are the complete policy universe for that evaluation. There is no concept of precedence, priority, or conflict resolution between bundles because there is only one.

---

## 7. Software Layering and Dependency Boundaries

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

The `runtime.PolicySource` interface is the contract between the engine and policy loading. It provides both policy content and source metadata:

```
PolicySource interface {
    LoadPolicy() (map[string][]byte, error)
    LoadData() ([]byte, error)
    PolicyRef() (string, error)
    BundleRevision() string
    ProfileName() string
}
```

The first three methods are the existing contract. The two new methods provide explicit metadata accessors for evidence binding:

| Method | `LocalFileSource` return | `BundleSource` return |
|---|---|---|
| `PolicyRef()` | SHA-256 content hash of module contents | `.manifest.revision` string |
| `BundleRevision()` | Empty string (`""`) | `.manifest.revision` string |
| `ProfileName()` | Empty string (`""`) | `.manifest.metadata.profile_name` string |

**Why explicit accessors instead of type assertions:** The engine MUST populate evidence fields (`bundle_revision`, `profile_name`) from `PolicySource.BundleRevision()` and `PolicySource.ProfileName()` directly. The engine MUST NOT type-assert `PolicySource` to a concrete type (e.g., `*BundleSource`) to access metadata. Type assertions would couple the engine to concrete implementations and introduce implicit branching on the policy source type.

**Why `EnvironmentLabel` is NOT on `PolicySource`:** The environment label is caller-supplied input context, not a property of the policy source. It flows through the engine's input path (`input.environment`), not through the `PolicySource` interface. Adding it to `PolicySource` would conflate input context with policy identity.

`pkg/bundlesource` implements this interface:
- `LoadPolicy` returns the extracted Rego modules from the archive.
- `LoadData` returns the data tree as produced by OPA's standard bundle loading merge behavior. Evidra MUST NOT implement custom data merge logic beyond what OPA's bundle loader provides natively.
- `PolicyRef` returns the `.manifest.revision` string.
- `BundleRevision` returns the `.manifest.revision` string.
- `ProfileName` returns the `.manifest.metadata.profile_name` string.

**Semantic distinction:** For `LocalFileSource`, `PolicyRef()` returns a SHA-256 content hash. For `BundleSource`, `PolicyRef()` returns the manifest revision. These are different kinds of identifiers. The engine treats `PolicyRef()` as an opaque string stamped onto decisions and MUST NOT use it for integrity or provenance decisions when `BundleRevision` is present. The evidence layer uses `BundleRevision()` (via the `PolicySource` interface) to determine policy identity authority: when the return value is non-empty, it is the authoritative policy identity; `PolicyRef` is secondary and informational.

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

Policy identity authority is determined exclusively by `BundleRevision()` return value (non-empty vs empty), not by the format or content of `PolicyRef()`. See §9 for the complete authority table.

**INVARIANT: Engine MUST NOT type-assert `PolicySource` to access metadata.**

The engine MUST NOT use Go type assertions, type switches, or interface-satisfaction checks on `PolicySource` to determine whether bundle metadata is available. The engine MUST use only the `PolicySource` interface methods. The `BundleRevision()` and `ProfileName()` methods return empty strings for non-bundle sources, which the engine records as-is. No conditional logic based on the concrete type of `PolicySource` is permitted.

`pkg/policysource.LocalFileSource` continues to satisfy the interface for development and testing. `LocalFileSource.BundleRevision()` returns `""`. `LocalFileSource.ProfileName()` returns `""`.

### New type: `BundleArtifact`

The bundle loader produces a `BundleArtifact` value that carries metadata beyond what `PolicySource` exposes:

| Field | Type | Source |
|---|---|---|
| `Revision` | `string` | `.manifest.revision` |
| `Roots` | `[]string` | `.manifest.roots` |
| `ProfileName` | `string` | `.manifest.metadata.profile_name` |
| `Modules` | `map[string][]byte` | Extracted `.rego` files |
| `Data` | `[]byte` | Merged JSON data documents (produced by OPA's standard bundle loader) |

`BundleArtifact` is immutable after construction. It must have no exported mutating methods and no exported mutable fields. All exported fields must be read-only (unexported fields set during construction, accessed via exported getter methods or exported as value types in a frozen struct).

**ProfileName derivation rule:** `ProfileName` must be derived from exactly one authoritative source: the `.manifest` file.

The derivation rule is:
1. If `.manifest.metadata.profile_name` is present and non-empty, use it as `ProfileName`.
2. If `.manifest.metadata.profile_name` is absent or empty, the bundle loader must return an error. `ProfileName` must not be inferred from the artifact filename, the directory name, or any other source.

There is exactly one derivation path. There is no fallback. Filename-based inference is prohibited because it couples the profile identity to deployment-time naming conventions that are outside the bundle's control.

---

## 8. Determinism Model

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
2. Evaluates a fixed input with a fixed environment label N times (N = 50)
3. Serializes each decision using canonical JSON (as defined above)
4. Asserts all N serialized outputs are byte-identical

This test must pass for every bundle artifact release. Failure is release-blocking.

---

## 9. Evidence Model Extension

### New fields on `EvidenceRecord`

The `evidence.EvidenceRecord` type gains four fields. These fields are populated from the `PolicySource` interface and the caller-provided input context.

| Field | Type | Source |
|---|---|---|
| `BundleRevision` | `string` | `PolicySource.BundleRevision()` — returns `.manifest.revision` for bundle sources, empty string for `LocalFileSource`. |
| `ProfileName` | `string` | `PolicySource.ProfileName()` — returns `.manifest.metadata.profile_name` for bundle sources, empty string for `LocalFileSource`. |
| `EnvironmentLabel` | `string` | Caller-provided environment label, recorded verbatim. Not sourced from `PolicySource`. |
| `InputHash` | `string` | SHA-256 hash of the canonical serialization of the input (`ToolInvocation` or `Scenario`). Enables input replay verification. |

For bundle-based evaluations, `BundleRevision` and `ProfileName` MUST be non-empty. The engine MUST populate these fields by calling `PolicySource.BundleRevision()` and `PolicySource.ProfileName()`, not by inspecting or parsing `PolicyRef()`.

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

`PolicyRef` is retained and continues to be populated from `PolicySource.PolicyRef()`.

### Policy identity authority rule

**INVARIANT:** When `BundleRevision` is non-empty, it is the sole authoritative policy identity. `PolicyRef` is informational only.

The following table defines which field governs each use case when `BundleRevision` is present:

| Use case | Authoritative field | `PolicyRef` role |
|---|---|---|
| Audit trail: "which policy produced this decision?" | `BundleRevision` | MUST NOT be used |
| Replay: "reproduce this decision" | `BundleRevision` + `InputHash` + `EnvironmentLabel` | MUST NOT be used |
| Provenance: "trace decision to release artifact" | `BundleRevision` | MUST NOT be used |
| Release lookup: "retrieve the artifact" | `BundleRevision` → Git tag → GitHub Release | MUST NOT be used |
| Content integrity: "were the bytes tampered with?" | Release artifact checksum + build/release pipeline | `PolicyRef` is informational only. For `LocalFileSource`, `PolicyRef` is a content hash and MAY serve as an integrity indicator. For `BundleSource`, `PolicyRef` returns the manifest revision (not a content hash) — content integrity in bundle mode is provided by the release artifact checksum and the build/release pipeline, not by `PolicyRef`. |

When `BundleRevision` is empty (`LocalFileSource` path for development and testing), `PolicyRef` remains the sole policy identity.

No system — engine, evidence consumer, reporting tool, or external integration — may use `PolicyRef` as policy identity when `BundleRevision` is non-empty. This is a non-optional invariant.

### Why binding revision is mandatory

Without `BundleRevision`, an evidence record says "a decision was made" but cannot answer "by which exact policy release?" The `PolicyRef` content hash is necessary but not sufficient — it proves what bytes were loaded but does not identify which release process produced them. `BundleRevision` closes this gap by linking every decision to a named, retrievable, reproducible artifact.

Without `InputHash`, an evidence record cannot be independently verified. Given the bundle artifact (identified by revision) and the input (identified by hash), any party can replay the evaluation and confirm the recorded decision is correct.

Without `EnvironmentLabel`, the replay is incomplete — the same input evaluated against the same bundle in different environments may produce different decisions (because environment-keyed param values differ). Recording the label makes the replay fully deterministic.

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
| 8 | Zero Go conditionals inspecting the environment label value | CI gate: grep/AST scan for `environment` in conditional expressions across `pkg/`, `cmd/`, `internal/` | Zero matches |
| 9 | Zero environment-name string literals in non-test Rego rule files | CI gate: pattern scan of `.rego` files under `evidra/policy/rules/` | Zero matches |
| 10 | Zero numeric literals in comparisons in deny/warn rule bodies | CI gate: pattern scan of `.rego` files under `evidra/policy/rules/` | Zero matches |
| 11 | Evidence record contains `bundle_revision` matching `PolicySource.BundleRevision()` | Integration test: evaluate via bundle, read evidence record | `record.bundle_revision == manifest.revision` (exact string match) |
| 12 | Evidence record contains `profile_name`, `environment_label`, `input_hash` | Integration test: evaluate via bundle with environment label | All three fields are non-empty strings |
| 13 | Determinism: repeated evaluations produce identical output | Determinism test: N = 50 evaluations with identical input, bundle, environment | All N serialized decisions (JSON with sorted keys) are byte-identical |
| 14 | Output ordering: `Hits`, `Hints`, `Reasons` are lexicographically sorted | Unit test: evaluate input that fires multiple rules | All three arrays are in ascending UTF-8 byte order |
| 15 | `BundleArtifact` has no exported mutating methods | Compilation review: exported method set inspection | Zero exported methods that modify struct fields |
| 16 | `pkg/bundlesource` satisfies `runtime.PolicySource` interface (including `BundleRevision()` and `ProfileName()`) | Compile-time interface satisfaction check | Compiles without error |
| 17 | `pkg/bundlesource` has no imports from `pkg/validate`, `pkg/policy`, or `cmd/*` | `go list -json ./pkg/bundlesource` import analysis | Zero forbidden imports |
| 18 | `pkg/evidence` has no imports from `pkg/policy`, `pkg/runtime`, or `pkg/bundlesource` | `go list -json ./pkg/evidence` import analysis | Zero forbidden imports |
| 19 | Engine passes environment label to OPA without modification | Unit test: set `input.environment` to arbitrary string, verify OPA receives identical string | `input.environment` in OPA input == caller-provided value (byte-identical) |
| 20 | Engine does not inject default environment when none is provided | Unit test: omit environment from invocation, verify OPA input has no `environment` key | `input.environment` is absent from OPA input document |
| 21 | No references to `data.evidra.data.thresholds` or `data.evidra.data.environments` in any Rego file | CI gate: grep scan of all `.rego` files | Zero matches |
| 22 | All tunable parameters exist under `data.evidra.data.params` | CI gate: data namespace audit — all param entries in `evidra/data/params/data.json`, no tunable values in other data files or in Rego/Go source | Zero violations |
| 23 | Unknown environment produces deterministic output per Parameter Resolution Contract | Integration test: evaluate with unlisted environment label | Decision is deterministic and matches expected fail-open or fail-closed behavior as documented by `unresolved_behavior` field |
| 24 | Missing param behavior is explicitly documented for every param entry | Build gate: every param key in `evidra/data/params/data.json` either has `by_env["default"]`, `safety_fallback`, or `unresolved_behavior` field | Zero undocumented param entries |
| 25 | Bundle loader rejects bundle with missing or empty `evidra/data/params/data.json` | Unit test: bundle archive without `evidra/data/params/data.json`, or with empty object | Returns error; no OPA evaluation occurs |
| 26 | Bundle data files are named `data.json` and placed in correct subdirectories | CI gate: verify no `.json` files under `evidra/data/` except in subdirectories as `data.json` | Zero incorrectly named data files |
| 27 | Engine populates evidence from `PolicySource.BundleRevision()` and `PolicySource.ProfileName()`, not from `PolicyRef()` parsing | Code review + unit test: verify evidence binding calls explicit accessors | No type assertions on `PolicySource`; no string parsing of `PolicyRef()` return value |
| 28 | `--profile` flag value validated against `.manifest.metadata.profile_name` | Unit test: `--profile` value differs from manifest `profile_name` | Returns error before evaluation |
| 29 | `LocalFileSource` implements `BundleRevision()` returning `""` and `ProfileName()` returning `""` | Compile-time check + unit test | Both methods return empty string; no error |
