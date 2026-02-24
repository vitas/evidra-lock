You are acting as a Principal Software Architect.

Task:
Produce a formal, implementation-ready specification document:

.ai/AI_CLAUDE_BUNDLE_ARTIFACT_SPEC.md

This document defines the complete contract for an Evidra OPA Bundle artifact.

The specification must formalize:

- Artifact identity
- Artifact structure
- Revision authority
- Canonical fingerprinting
- Replay linkage
- Integrity assumptions
- Deterministic guarantees

No code.
No pseudo-code.
No SaaS discussion.
No MCP discussion.
No marketing language.

Strict engineering tone.

------------------------------------------------------------
SECTION 1 — Purpose
------------------------------------------------------------

Define:

- What an Evidra Bundle Artifact is
- Why it is the authoritative policy unit
- Why artifact identity must be immutable
- How it connects build → release → evaluation → evidence → replay

------------------------------------------------------------
SECTION 2 — Artifact Identity Model
------------------------------------------------------------

Define three identity layers:

1) Manifest Revision (authoritative identity)
2) Artifact Checksum (content fingerprint)
3) Git Tag (release locator)

Formally define:

- BundleRevision = manifest.revision
- ArtifactChecksum = SHA-256 of canonical .tar.gz
- GitTag = release identifier in repository

Define the invariant:

1 GitTag → 1 ManifestRevision → 1 ArtifactChecksum → 1 Immutable Artifact

Define all invalid states:
- Reused revision
- Mismatched revision and filename
- Re-tagging
- Re-publishing different artifact under same revision

------------------------------------------------------------
SECTION 3 — Canonical Artifact Structure
------------------------------------------------------------

Define exact archive layout:

Root:
  .manifest
  evidra/
    policy/
    data/

Define:

- No extra files
- No top-level wrapper directory
- .manifest must be at archive root
- All Rego packages under evidra.policy
- All JSON data under evidra/data/

Define structural validation requirements.

------------------------------------------------------------
SECTION 4 — Manifest Contract
------------------------------------------------------------

Define required fields:

- revision (non-empty, immutable)
- roots (must equal ["evidra"])
- metadata.profile_name (mandatory, sole source of profile identity)

Define:

- revision format requirements
- immutability rule
- placeholder rules in repository
- injection at build time

Prohibit:

- runtime revision inference
- filename-based profile derivation
- default manifest values

------------------------------------------------------------
SECTION 5 — Canonical Fingerprint Definition
------------------------------------------------------------

Define:

ArtifactChecksum = SHA-256 over canonical .tar.gz

Define canonical build constraints:

- Deterministic tar
- Fixed timestamps
- Sorted entries
- Normalized permissions
- Deterministic gzip implementation
- No embedded metadata

Define:

Checksum mismatch = artifact integrity violation.

------------------------------------------------------------
SECTION 6 — Artifact Immutability
------------------------------------------------------------

Define:

- Once published, artifact content must never change.
- Revision reuse is prohibited.
- Rebuilding from same Git tag must yield identical checksum.
- Artifact content must not depend on build machine state.

Define enforcement:

- Deterministic rebuild test
- Release-blocking on mismatch

------------------------------------------------------------
SECTION 7 — Artifact ↔ Evidence Binding
------------------------------------------------------------

Define binding contract:

Evidence record must contain:
- bundle_revision
- environment_label
- input_hash

Replay identity is defined by:

(bundle_revision, input_hash, environment_label)

Define:

Replay protocol (conceptual):
1) Retrieve artifact by revision.
2) Verify artifact checksum.
3) Reconstruct input from canonical serialization.
4) Re-evaluate.
5) Compare canonical decision output.

Define that replay must be byte-identical.

------------------------------------------------------------
SECTION 8 — Artifact Trust Assumptions
------------------------------------------------------------

Define trust boundaries:

- Git repository integrity
- Git tag integrity
- GitHub Release integrity
- Checksum verification

Define non-goals:

- No artifact signing (out of scope)
- No remote registry
- No dynamic fetching
- No runtime mutation

------------------------------------------------------------
SECTION 9 — Determinism Guarantees
------------------------------------------------------------

Formally state:

Given identical:
- ArtifactChecksum
- EnvironmentLabel
- Canonical Input JSON

Decision output must be byte-identical.

State that determinism covers:
- Boolean result
- All arrays sorted
- Canonical JSON serialization
- No time dependency
- No randomness
- No external I/O

------------------------------------------------------------
SECTION 10 — Failure Conditions
------------------------------------------------------------

Define all conditions that must cause hard failure:

- Missing .manifest
- Empty revision
- Incorrect roots
- Checksum mismatch
- Determinism test failure
- Multiple bundles supplied
- Attempt to reuse revision

All failures must be release-blocking.

------------------------------------------------------------
SECTION 11 — Explicit Non-Goals
------------------------------------------------------------

State clearly:

- No multi-bundle composition
- No layered artifacts
- No environment registry
- No SaaS
- No policy marketplace
- No runtime editing

------------------------------------------------------------
OUTPUT REQUIREMENTS
------------------------------------------------------------

- Structured headings
- Formal invariant language (MUST / MUST NOT)
- No code blocks
- No pseudo-code
- No examples unless structural (file tree allowed)
- Must read as a normative specification
- Must be implementation-ready