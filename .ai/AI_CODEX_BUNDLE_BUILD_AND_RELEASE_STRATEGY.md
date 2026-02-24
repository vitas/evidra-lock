# AI Bundle Build and Release Strategy

## 1. Bundle Source Layout

### Terminology
- **Bundle artifact**: released OPA bundle archive.
- **Profile**: policy family encoded in a bundle artifact.
- **Policy roots**: namespace roots declared in `.manifest.roots`.

### Repository Structure
- Policy source is organized to match OPA policy roots.
- Rego modules and JSON data are co-located under explicit namespace ownership.
- Bundle artifact build input excludes unrelated repository content.

### Versioning Conventions
- Bundle version is represented by manifest revision.
- `bundle_revision` is the authoritative artifact identity.
- Git tag is a release locator and must map 1:1 to `bundle_revision`.
- Artifact checksum is an integrity proof and must correspond to the artifact that declares that `bundle_revision`.
- One `bundle_revision` maps to one immutable artifact payload.

## 2. Bundle Build Process

### Validation Steps
- Validate Rego parse and policy tests.
- Validate data schema and required keys.
- Validate policy roots ownership and namespace boundaries.
- Validate manifest presence and completeness.

### Manifest Revision Requirements
- `.manifest.revision` must be present and non-empty.
- Revision must be generated from release identity, not runtime state.
- Revision must be stable across rebuilds of the same source for the same profile.
- Manifest revision must match release artifact naming and release metadata exactly.
- Re-tagging different content under an existing manifest revision is forbidden.

### Deterministic Build Expectations
- Same source revision must produce byte-identical bundle artifact payload.
- Build process must normalize archive ordering and metadata to eliminate non-deterministic output.
- Build metadata that changes per machine/time must not alter policy artifact content.
- Deterministic build checks are required and release-blocking.

## 3. Versioning Strategy

- Revision format must be monotonic and traceable to Git history.
- Release tag identifies policy release.
- Manifest revision encodes release tag and immutable commit identity.
- Revision is the canonical runtime identity; display versions are secondary.
- `policy_ref` is informational only and must not be used as canonical artifact identity.

## 4. GitHub Release Model

### Artifact Naming Conventions
- `evidra-policy-<profile>-<revision>.tar.gz`
- Optional companion checksum:
  - `evidra-policy-<profile>-<revision>.sha256`
- `<revision>` in filename must equal `.manifest.revision` exactly.

### Packaging
- Artifact format is standard gzip-compressed tar archive compatible with OPA bundle loading.
- Archive root contains `.manifest` and all policy/data paths claimed by policy roots (manifest roots).

### Release Notes Policy
- Release notes include:
  - manifest revision
  - policy scope/profile
  - compatibility statement
  - policy behavior change summary
- Release notes must explicitly list breaking policy changes.
- Release publication is blocked on any mismatch between release notes revision and manifest revision.

## 5. Integrity and Reproducibility

### Revision-Artifact Binding
- Bundle artifact filename revision, manifest revision, and release metadata must match exactly.
- Any mismatch is a release-blocking integrity failure.
- Artifact immutability is absolute after release publication.
- Replacing bytes of an already-published artifact is forbidden.

### Deterministic Rebuild
- Auditor must be able to rebuild from tagged source and obtain matching checksum.
- Reproducibility is mandatory for evidence verification.
- Reproducibility verification is required before release publication.

### Validation Before Publishing
- Pre-release gate requires:
  - bundle artifact load validation
  - manifest revision and artifact-name equivalence validation
  - namespace ownership validation against manifest roots
  - policy decision contract checks
  - manifest consistency checks
  - checksum generation
  - deterministic rebuild checksum equivalence validation

## 6. Non-Goals

- No remote bundle registry adoption in this architecture scope.
- No dynamic runtime bundle artifact downloads.
- No runtime artifact mutation.
- No multi-bundle packaging or composition.

## 7. Refinement Summary

### Removed Ambiguities
- Removed ambiguity between revision, Git tag, and checksum roles.
- Removed ambiguity on artifact mutability after release publication.
- Removed ambiguity on revision matching by requiring strict equality across manifest, filename, and release metadata.

### Clarified Invariants
- `bundle_revision` is authoritative; `policy_ref` is informational.
- Re-tagging under the same revision is forbidden.
- Deterministic rebuild validation is mandatory and release-blocking.
