# Bundle Build and Release Strategy

**Role:** Principal Software Architect  
**Status:** Approved  
**Topic:** OPA Bundle Artifact Lifecycle  

## 1. Bundle Source Layout

The policy repository must maintain a strict directory structure that maps directly to the OPA bundle layout requirements. 

*   **Repository Structure:** Source code must separate Rego policy files (`.rego`) from data files (`.json`) within a defined hierarchy representing the data namespace.
*   **Versioning Conventions:** The repository will utilize Git tags as the primary source of truth for versioning. The structure must allow the build process to cleanly package the root policy directory without including repository metadata (e.g., `.git`).

## 2. Bundle Build Process

The transformation from source code to a deployable artifact is a deterministic build process.

*   **Validation Steps:** Before packaging, the build process must invoke `opa check` to ensure syntax validity and `opa test` to verify logic against the data namespace.
*   **Manifest Revision Requirements:** The build process is responsible for generating or updating the `.manifest` file. The `revision` field in the manifest must be dynamically injected during the build to match the commit SHA or the Git tag being built.
*   **Deterministic Build Expectations:** The packaging step must produce a deterministic tarball. This requires normalizing file timestamps and ensuring consistent file ordering within the archive so that rebuilding the same Git commit yields an identical SHA-256 checksum for the `.tar.gz` file.

## 3. Versioning Strategy

*   **Manifest Revision Format:** The `revision` field within the `.manifest` must utilize Semantic Versioning (SemVer) if built from a tag, or a Git short-SHA if built from a development branch.
*   **Mapping to Git Tags:** Every official release of a bundle artifact must correspond 1:1 with a signed Git tag in the source repository. The tag name dictates the bundle's revision identity.

## 4. GitHub Release Model

Bundle artifacts will be distributed via standard Version Control System release mechanisms.

*   **Artifact Naming Conventions:** The output artifact must follow a strict naming convention: `bundle-<profile_name>-<version>.tar.gz`.
*   **Tar.gz Bundle Packaging:** The artifact is a standard Gzip-compressed tar archive containing the `.manifest` at the root, alongside the policy and data directories.
*   **Checksum:** The release must include a `checksums.txt` file containing the SHA-256 hash of the `.tar.gz` artifact.
*   **Release Notes Policy:** Every release must include automated release notes detailing the diff of policy logic and data namespace changes since the previous revision.

## 5. Integrity & Reproducibility

*   **Revision Matching:** The `revision` declared inside the bundle's `.manifest` must exactly match the version identifier in the artifact's filename and the corresponding GitHub Release tag. A mismatch is considered a critical integrity failure.
*   **Deterministic Builds:** Because the evidence record logs the `bundle_revision`, auditors must be able to check out the corresponding Git tag, run the build process, and produce an artifact with the exact same checksum. If builds are not deterministic, cryptographic trust in the audit trail is broken.
*   **Validation Before Publishing:** The CI pipeline must perform a test load of the generated `.tar.gz` bundle using the Evidra engine before publishing the artifact to the release page.

## 6. Non-Goals

To maintain architectural simplicity and security, the following are explicitly out of scope for this strategy:

*   **No Remote Registry:** The system will not push to or pull from OCI registries or proprietary policy servers.
*   **No Dynamic Runtime Downloads:** Evidra will not fetch bundles over the network at execution time. The `.tar.gz` artifact must be supplied locally to the CLI or execution engine.