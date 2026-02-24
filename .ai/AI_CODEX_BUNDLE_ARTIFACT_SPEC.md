# Evidra OPA Bundle Artifact Specification (CODEX)

## 1. Purpose
This specification defines the normative contract for an Evidra OPA Bundle Artifact.

An Evidra Bundle Artifact is the sole authoritative policy unit for build, release, evaluation, evidence recording, and replay. The artifact is a complete and immutable packaging of policy logic and policy data.

Artifact identity MUST be immutable to preserve auditability and replay integrity. The same policy identity MUST map to the same artifact content across time.

This specification defines the required continuity chain from build to replay:
- Build produces one canonical artifact.
- Release publishes that artifact with a stable identity.
- Evaluation consumes exactly that artifact.
- Evidence records the artifact identity and replay keys.
- Replay re-evaluates against the same artifact identity and canonical input representation.

## 2. Artifact Identity Model
The artifact identity model consists of three distinct identity layers:
- Manifest Revision: authoritative identity.
- Artifact Checksum: content fingerprint.
- Git Tag: release locator.

Normative definitions:
- BundleRevision = manifest.revision.
- ArtifactChecksum = SHA-256 digest of the canonical artifact `.tar.gz`.
- GitTag = release identifier in the source repository.

Normative invariant:
- One GitTag MUST map to exactly one BundleRevision.
- One BundleRevision MUST map to exactly one ArtifactChecksum.
- One ArtifactChecksum MUST map to exactly one immutable artifact payload.

Equivalent invariant form:
- 1 GitTag -> 1 ManifestRevision -> 1 ArtifactChecksum -> 1 Immutable Artifact.

Invalid states:
- Reused revision: the same BundleRevision mapped to more than one checksum.
- Revision/filename mismatch: artifact filename revision does not equal manifest.revision.
- Re-tagging: moving an existing GitTag to different content.
- Re-publishing drift: publishing a different artifact payload under an existing BundleRevision.

All invalid states MUST be treated as integrity violations.

## 3. Canonical Artifact Structure
The archive layout is normative and mandatory.

Root entries:
- `.manifest`
- `evidra/`

Required subtree:
- `evidra/policy/`
- `evidra/data/`

Structural rules:
- `.manifest` MUST exist at archive root.
- The archive MUST NOT contain a top-level wrapper directory.
- The archive MUST NOT contain extra top-level files beyond `.manifest` and `evidra/`.
- All Rego modules MUST belong to package namespace `evidra.policy`.
- All JSON policy data MUST reside under `evidra/data/`.
- Policy data outside `evidra/data/` MUST be rejected.
- Policy modules outside `evidra/policy/` MUST be rejected.

Structural validation MUST fail hard on any deviation.

## 4. Manifest Contract
The manifest contract is normative.

Required fields:
- `revision`: non-empty string; immutable identity.
- `roots`: exact value `["evidra"]`.
- `metadata.profile_name`: mandatory and authoritative profile identifier.

Revision requirements:
- `revision` MUST be present and MUST be non-empty.
- `revision` MUST be immutable after publication.
- `revision` MUST be injected during build/release preparation.
- Repository placeholders for revision MUST NOT be treated as publishable values.

Profile requirements:
- `metadata.profile_name` MUST be present and MUST be non-empty.
- `metadata.profile_name` MUST be the sole profile identity source.
- Profile identity MUST NOT be inferred from filename, directory name, or runtime path.

Prohibitions:
- Runtime revision inference is prohibited.
- Filename-based profile derivation is prohibited.
- Default manifest values are prohibited.

## 5. Canonical Fingerprint Definition
ArtifactChecksum is defined as SHA-256 over the canonical `.tar.gz` artifact bytes.

Canonical build constraints:
- Archive entry order MUST be deterministic and lexicographically sorted.
- Entry timestamps MUST be fixed and deterministic.
- File permissions MUST be normalized.
- Tar header metadata MUST be normalized and deterministic.
- Gzip output MUST be deterministic.
- Embedded volatile metadata MUST be excluded.

Any checksum mismatch MUST be treated as an artifact integrity violation.

## 6. Artifact Immutability
Immutability is mandatory.

Immutability rules:
- Once published, artifact content MUST NEVER change.
- BundleRevision reuse for different content MUST NEVER occur.
- Rebuilding from the same GitTag MUST produce the same ArtifactChecksum.
- Artifact content MUST NOT depend on machine-specific state, local clock, filesystem order, locale, or environment-dependent metadata.

Enforcement requirements:
- Deterministic rebuild verification MUST run before release publication.
- Any checksum mismatch during deterministic rebuild verification MUST block release.

## 7. Artifact to Evidence Binding
Evidence binding is mandatory and normative.

Each evidence record MUST contain:
- `bundle_revision`
- `environment_label`
- `input_hash`

Replay identity is defined as the tuple:
- (bundle_revision, input_hash, environment_label)

Replay protocol requirements:
- The artifact MUST be resolved by `bundle_revision`.
- The resolved artifact checksum MUST be verified against expected ArtifactChecksum.
- Input MUST be reconstructed from canonical serialization.
- Evaluation MUST run against the resolved artifact and canonical input.
- Replay output MUST be compared against canonical decision output.

Replay result MUST be byte-identical to the original canonical decision output.

## 8. Artifact Trust Assumptions
Trust boundaries:
- Source repository integrity.
- GitTag integrity.
- Release artifact integrity.
- ArtifactChecksum verification integrity.

Operational trust requirements:
- ArtifactChecksum verification MUST occur before evaluation and before replay verification.
- Release metadata MUST remain consistent with manifest identity and artifact checksum.

Out-of-scope trust mechanisms:
- Artifact signing is out of scope.
- Remote registry distribution is out of scope.
- Dynamic runtime fetching is out of scope.
- Runtime artifact mutation is out of scope.

## 9. Determinism Guarantees
Determinism guarantee:
- Given identical ArtifactChecksum, EnvironmentLabel, and Canonical Input JSON, decision output MUST be byte-identical.

Determinism scope MUST include:
- Boolean decision result.
- Ordered arrays with deterministic sort.
- Canonical JSON serialization.
- Stable key ordering in serialized output.

Determinism exclusions:
- Time-dependent behavior is prohibited.
- Randomness is prohibited.
- External I/O side effects in evaluation are prohibited.
- Non-deterministic iteration order dependence is prohibited.

Array ordering requirements:
- Violations array MUST be deterministically sorted by rule identifier, then message.
- Rule identifier array MUST be deterministically sorted.
- Hints array MUST be deterministically sorted after de-duplication.

## 10. Failure Conditions
The following conditions MUST cause hard failure:
- Missing `.manifest`.
- Empty `manifest.revision`.
- Incorrect `manifest.roots`.
- Missing `metadata.profile_name`.
- ArtifactChecksum mismatch.
- Determinism verification failure.
- Multiple bundle inputs supplied for a single execution.
- Attempted revision reuse.
- Revision and filename mismatch.

Failure policy:
- All listed failures are release-blocking.
- Evaluation MUST NOT proceed when artifact contract validation fails.
- Replay verification MUST fail on any contract violation.

## 11. Explicit Non-Goals
The following are explicitly out of scope:
- Multi-bundle composition.
- Layered artifacts.
- Environment registry systems.
- Hosted policy distribution models.
- Policy marketplace concepts.
- Runtime policy editing.

These non-goals MUST NOT be introduced into this artifact contract.
