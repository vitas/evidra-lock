# Bundle Build and Release Strategy

**Date:** 2026-02-24
**Author:** Claude Opus 4.6 (Principal Architect role)
**Status:** Draft (Refined 2026-02-24)
**Scope:** Build, versioning, release, and integrity model for Evidra OPA bundle artifacts

---

## 1. Bundle Source Layout

### Repository Structure

Policy source lives in the existing repository under a directory that maps directly to the OPA bundle internal layout. The build process must package this directory into a `.tar.gz` archive without transformation of the directory structure. The archive must contain only the `.manifest` file and the `evidra/` subtree. No other files from the repository are permitted in the archive.

```
policy/
  bundles/
    ops-v0.1/                              # One directory per profile
      .manifest                            # Bundle manifest (revision injected at build time)
      evidra/
        policy/
          decision.rego                    # Aggregator: data.evidra.policy.decision
          defaults.rego                    # Shared helpers (has_tag, action_namespace)
          rules/
            deny_kube_system.rego
            deny_prod_without_approval.rego
            deny_public_exposure.rego
            deny_mass_delete.rego
            warn_breakglass.rego
            warn_autonomous_execution.rego
        data/
          thresholds.json                  # Environment-keyed thresholds
          rule_hints.json                  # Rule ID to hint mapping
          environments.json                # Per-environment namespace config
      tests/                               # OPA tests (excluded from bundle archive)
        deny_kube_system_test.rego
        deny_prod_without_approval_test.rego
        ...
```

Key structural rules:
- The `evidra/` directory is the bundle root content. Everything inside it maps to the `data.evidra` OPA namespace.
- The `tests/` directory is a sibling of `evidra/`, not inside it. Tests are excluded from the bundle archive by the build process.
- The `.manifest` file sits at the same level as `evidra/`, inside the profile directory. It is included in the archive root.
- No files outside `policy/bundles/<profile>/` are included in any bundle artifact.

### Versioning Conventions

- Each profile directory represents a policy family (e.g., `ops-v0.1`).
- The profile name is a stable identifier that does not change across revisions of the same policy family.
- The manifest `revision` is the per-release identity. It changes with every release.
- Git tags are the source of truth for release identity. A tag on the repository triggers the bundle build.

---

## 2. Bundle Build Process

### Build steps (ordered)

| Step | Action | Failure behavior |
|---|---|---|
| 1. Parse | Validate all `.rego` files parse successfully (`opa check`) | Abort build |
| 2. Test | Run all OPA tests in the `tests/` directory (`opa test`) | Abort build |
| 3. Data validation | Validate that all JSON data files parse as valid JSON and contain required top-level keys | Abort build |
| 4. Manifest injection | Write `.manifest` with `revision` set to the release identifier and `roots` set to `["evidra"]` | Abort build if revision is empty |
| 5. Namespace validation | Verify all Rego packages are under `evidra.policy` and all data paths are under `evidra/data/` | Abort build |
| 6. Archive | Create `.tar.gz` from the profile directory contents (`.manifest` + `evidra/`), excluding `tests/` | Abort on I/O error |
| 7. Checksum | Compute SHA-256 of the archive and write to a companion `.sha256` file | Abort on I/O error |
| 8. Smoke test | Load the produced archive using the bundle loader and evaluate a known test input | Abort if decision does not match expected output |

### Manifest revision requirements

- The `revision` field must be non-empty.
- The `revision` value must be derived from the release identity (Git tag + short commit hash), not from build-time state such as timestamps or hostnames.
- The `revision` must be injected during the build process, not maintained manually in source control.

### Manifest injection integrity rule

The `.manifest` file in the repository MUST NOT contain a production revision value. The repository copy must contain a placeholder value (exactly `"dev"`) that is overwritten by the build process at archive creation time. This prevents accidental use of the repository `.manifest` as a release artifact.

The build process must enforce the following injection sequence:
1. Read the Git tag to derive the target revision string.
2. Overwrite the `revision` field in `.manifest` with the derived value.
3. Overwrite the `metadata.profile_name` field if not already present.
4. Verify the resulting manifest passes all INV-7 validation checks.
5. Proceed to archive creation.

**CI gate:** On every pull request that modifies any `.manifest` file under `policy/bundles/`, CI must verify that the `revision` field equals exactly `"dev"`. A pull request that introduces a non-placeholder revision value into the repository must be rejected. This prevents accidental commit of a release revision that would break the injection model.

**Triple-match requirement:** At release time, the following three values must be identical (excluding the commit hash suffix in the manifest revision):
- The semver extracted from the Git tag (e.g., `v0.1.0` from `policy/ops-v0.1/v0.1.0`)
- The semver portion of the manifest revision (e.g., `0.1.0` from `ops-v0.1.0-a3f8c91`)
- The semver in the artifact filename (e.g., `0.1.0` from `evidra-policy-ops-v0.1-0.1.0.tar.gz`)

A mismatch among any of these three is a release-blocking failure.

### Deterministic build requirements

The build process must produce a byte-identical `.tar.gz` archive when run against the same source commit, regardless of:
- The machine performing the build
- The wall-clock time of the build
- The filesystem ordering of the host
- The operating system of the build host

The following parameters are mandatory for deterministic output:

| Parameter | Required value |
|---|---|
| Archive format | POSIX (pax) tar |
| File modification timestamps | Fixed epoch: `2000-01-01T00:00:00Z` (UTC) |
| File owner UID/GID | `0`/`0` (root) |
| File permissions | `0644` for regular files, `0755` for directories |
| Entry ordering | Lexicographic ascending by full path (UTF-8 byte order) |
| Compression algorithm | gzip (RFC 1952) |
| Compression level | 6 (Go `compress/flate` default — see gzip determinism note below) |
| gzip header: mtime | Fixed: `0` (Unix epoch, per RFC 1952 §2.3: "0 means no time stamp is available") |
| gzip header: filename (FNAME) | Not set (FNAME flag clear) |
| gzip header: OS field | `0xFF` (unknown, per RFC 1952 §2.3) |
| gzip header: extra/comment | Not set |
| Build metadata in archive | None. No comments, no extended attributes, no machine-specific headers. |

**Deterministic gzip note:** Cross-implementation gzip determinism is not guaranteed — different gzip libraries (zlib, Go stdlib, pigz) produce different output even at the same compression level due to implementation-specific heuristics in the DEFLATE algorithm. To guarantee byte-identical archives, the build process MUST use a single controlled implementation. The mandated implementation is the Go standard library (`compress/gzip` with `compress/flate`). Compression level 6 is specified because it is the Go `compress/flate` default (`flate.DefaultCompression`), ensuring that builds using default settings produce identical output. Builds MUST NOT use external gzip binaries, alternative Go compression libraries, or CGo wrappers around zlib.

Determinism is verified by rebuilding from the same Git tag on a clean checkout using the mandated Go implementation and comparing the SHA-256 checksum of the produced archive to the published checksum. A mismatch is a release-blocking failure.

---

## 3. Versioning Strategy

### Manifest revision format

The manifest revision encodes both the human-readable version and the immutable commit identity:

```
<profile>-<semver>-<short-commit-hash>
```

Examples:
- `ops-v0.1.0-a3f8c91` — first release of `ops-v0.1` profile
- `ops-v0.1.1-b7d2e03` — patch release
- `ops-v0.2.0-c1e9f45` — minor release with new rules

The semver portion follows Semantic Versioning:
- **Major:** Breaking changes to the decision contract (new required input fields, removed rules that downstream systems depend on)
- **Minor:** New rules, new data keys, expanded environment support
- **Patch:** Threshold adjustments, hint text changes, bug fixes in existing rules

The short commit hash (7 characters) is the first 7 hex characters of the full Git commit SHA. It provides immutable traceability without requiring the full 40-character hash.

### Mapping to Git tags

| Artifact | Git tag | Manifest revision |
|---|---|---|
| `evidra-policy-ops-v0.1-0.1.0.tar.gz` | `policy/ops-v0.1/v0.1.0` | `ops-v0.1.0-a3f8c91` |
| `evidra-policy-ops-v0.1-0.1.1.tar.gz` | `policy/ops-v0.1/v0.1.1` | `ops-v0.1.1-b7d2e03` |

Git tags use the path-prefixed format `policy/<profile>/v<semver>` to distinguish policy releases from binary releases in the same repository.

One Git tag maps to exactly one manifest revision. One manifest revision maps to exactly one immutable artifact. This is a 1:1:1 relationship. The following are unconditional integrity violations:
- Re-tagging: deleting and recreating a Git tag pointing to a different commit
- Re-publishing: uploading a different artifact under an existing revision
- Revision aliasing: two different revisions resolving to the same artifact content (permitted but discouraged)

There is no exception mechanism for these violations. A violated 1:1:1 mapping requires a new revision.

---

## 4. GitHub Release Model

### Artifact naming conventions

Release artifacts follow a strict naming convention:

```
evidra-policy-<profile>-<semver>.tar.gz
evidra-policy-<profile>-<semver>.sha256
```

Examples:
```
evidra-policy-ops-v0.1-0.1.0.tar.gz
evidra-policy-ops-v0.1-0.1.0.sha256
```

The profile name in the filename matches the profile directory name in the repository. The semver in the filename matches the semver portion of the manifest revision (without the commit hash suffix).

### Archive packaging

The `.tar.gz` archive contains the bundle content rooted at the archive root:

```
.manifest
evidra/
  policy/
    decision.rego
    defaults.rego
    rules/
      ...
  data/
    thresholds.json
    rule_hints.json
    environments.json
```

The archive does not contain:
- The profile directory name as a top-level prefix (i.e., no `ops-v0.1/` wrapper)
- Test files
- Repository metadata (`.git`, `CLAUDE.md`, `ai/`)
- Build scripts or Makefiles

This layout is required for OPA bundle loading compatibility. `.manifest` must be at the archive root. Policy and data must be under the declared roots. Any deviation from this layout is a build failure.

### Checksum file format

The `.sha256` file contains a single line:

```
<hex-encoded SHA-256>  evidra-policy-<profile>-<semver>.tar.gz
```

This follows the GNU coreutils `sha256sum` format, enabling verification with `sha256sum -c`.

### Release notes policy

Every GitHub Release must include:
- The manifest revision (full string including commit hash)
- The profile name
- A summary of policy behavior changes since the previous revision
- A list of new, modified, or removed rule IDs
- A list of data namespace changes (new keys, changed thresholds)
- A compatibility statement: whether the release is backward-compatible with the previous revision's input schema
- The SHA-256 checksum of the artifact

Breaking changes (rule removals, input schema changes, decision contract changes) must be called out explicitly with a "BREAKING" label.

---

## 5. Integrity and Reproducibility

### Why revision must match artifact

The evidence record's `bundle_revision` field is the sole link between a recorded decision and the policy that produced it. If the manifest revision inside the artifact does not match the filename and Git tag, then:
- Evidence records cannot be traced to a retrievable artifact
- Deterministic replay is impossible
- Audit integrity is broken

Therefore: the manifest revision, the artifact filename version, and the Git tag must all correspond. The build process must verify this correspondence before publishing. A mismatch is a release-blocking failure.

### Why builds must be deterministic

Deterministic builds serve two purposes:

1. **Audit verification.** An auditor must be able to check out the Git tag, run the build, and produce an artifact with the same SHA-256 checksum as the published artifact. If the build is non-deterministic, the auditor cannot verify that the published artifact was produced from the claimed source.

2. **Cache safety.** If two builds from the same source produce different checksums, downstream systems cannot cache or deduplicate artifacts by checksum. This leads to unnecessary storage and confusion about artifact identity.

The build process achieves determinism by:
- Using a fixed timestamp for all archive entries
- Sorting archive entries by path
- Using deterministic compression parameters
- Not embedding build-time metadata in the archive

### Validation before publishing

The release pipeline must execute the following gates before publishing an artifact to a GitHub Release:

All gates are mandatory and release-blocking. No gate is advisory.

| Gate | Description | Pass condition |
|---|---|---|
| Manifest consistency | `revision` in `.manifest` matches the value derived from the Git tag | Exact string match |
| Manifest roots | `.manifest.roots` equals `["evidra"]` | Exact match; single element |
| Bundle loadability | The archive loads via the bundle loader | No errors returned |
| Decision contract | A known test input produces the expected decision output | All decision fields match expected values |
| Determinism | Two independent builds from the same tag produce identical archives | SHA-256 checksums are identical |
| OPA tests pass | All test files in the `tests/` directory pass | Zero test failures |
| No forbidden patterns | No environment literals in Rego, no hardcoded thresholds in rule files | Zero pattern matches |
| Checksum file present | `.sha256` file is generated and included in the release | File exists and contains valid checksum |

---

## 6. Non-Goals

The following are explicitly out of scope for this build and release strategy:

| Non-goal | Rationale |
|---|---|
| **Remote bundle registry** (OCI, S3, custom registry) | Adds operational complexity. Bundles are distributed as GitHub Release artifacts. |
| **Dynamic runtime downloads** | The engine does not fetch bundles over the network. The bundle path is a local filesystem path provided by the caller. |
| **Runtime artifact mutation** | Once loaded, the bundle artifact is immutable for the duration of the evaluation. Hot-reloading or patching is not supported. |
| **Multi-bundle packaging** | Each artifact contains exactly one profile's policy. There is no "uber-bundle" combining multiple profiles. |
| **Signed bundles** | Not in scope. Checksum verification provides integrity assurance for MVP. The archive format is compatible with signing adoption without structural changes. |
| **Automatic version bumping** | The version is set explicitly by the release author via Git tag. There is no automated semver increment. |
