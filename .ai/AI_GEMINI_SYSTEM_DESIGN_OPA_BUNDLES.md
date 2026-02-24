# System and Software Architecture: OPA Bundles as Primary Policy Artifact

**Role:** Principal Software Architect  
**Status:** Approved  
**Topic:** OPA Bundle Integration for Evidra Policy Definition  

## 1. Architectural Overview

### Current State
Evidra currently relies on a loose collection of Rego files and a standalone `data.json` file. Policies and data are supplied directly to the OPA engine at runtime. This approach lacks a formalized packaging mechanism, making versioning, distribution, and strict integrity verification challenging. Environments are often handled through fragmented data files or ad-hoc Rego logic.

### Target State
Evidra will mandate the official **OPA Bundle** format as the singular, authoritative mechanism for policy definition, packaging, and execution. The system will transition to loading a strictly structured, compressed tarball (`.tar.gz`) containing a `.manifest`, policies, and namespaced data. 

### Text Diagram
```text
[CLI Invocation]
       |
       v
[Execution Engine] ---> (Parses inputs, opaque environment label)
       |
       v
[Bundle Loader] ------> (Extracts single .tar.gz, validates .manifest)
       |
       v
[OPA Evaluation] -----> (Executes policy against data namespace using environment label)
       |
       v
[Evidence Store] -----> (Records decision, input_hash, bundle_revision, environment_label)
```

## 2. Strategic Decision: OPA Bundle as Sole Policy Artifact

The decision to adopt the official OPA Bundle format is rooted in standardization and ecosystem compatibility. By adopting the standard, Evidra avoids the technical debt of maintaining a custom packaging DSL or proprietary archive formats. The `.manifest` within the bundle provides a definitive `revision`, which serves as the authoritative identifier for the policy state. This revision guarantees that any evaluation can be cryptographically linked to a specific, immutable point in the policy lifecycle.

## 3. Bundle-Based Policy Architecture

The architecture dictates a strict internal structure for the OPA bundle.

*   **Required Directory Structure:** The bundle must adhere to OPA's standard layout, segregating Rego modules from JSON data files.
*   **Manifest Requirements:** A `.manifest` file is strictly required. It must define the bundle `revision` and specify the roots that the bundle claims authority over.
*   **Data Namespace Isolation:** All policy data (e.g., thresholds, allowlists) must be strictly namespaced within the bundle's data hierarchy. Global or un-namespaced data is an architectural violation.

## 4. Data-Driven Environment Model

Evidra must operate in a fully data-driven manner regarding environments.

*   **Opaque String:** The "environment" is treated strictly as an opaque string label provided at invocation (e.g., "prod", "staging", "eu-west").
*   **No Fixed Enum:** The Go runtime and the Rego engine must not maintain an enumeration of valid environments.
*   **No Rego Literals:** Rego policies must not contain inline string literals for environment matching (e.g., `if env == "prod"`).
*   **No Go Branching:** The Go execution engine must not contain conditional logic based on the environment label.
*   **Deterministic Resolution:** The execution engine passes the opaque environment label to the OPA evaluation context. The Rego policy utilizes this label strictly as a key to perform a lookup within the namespaced data object. The data object holds the threshold or configuration specific to that label. If the label is not found in the data namespace, the policy must fail closed deterministically.

## 5. Single-Bundle Execution Model

To guarantee strict determinism and simplify the evidence trail, Evidra employs a Single-Bundle Execution Model.

*   **Exactly One Bundle:** The execution engine will load and evaluate exactly one OPA bundle per invocation.
*   **No Composition:** Multi-bundle composition, layering, or overlaying is strictly prohibited. 
*   **No Merge Strategy:** The system will not attempt to merge data or rules from multiple sources. The single loaded bundle represents the entirety of the policy universe for that execution.

## 6. Software Layering & Dependency Boundaries

The architecture enforces strict unidirectional dependency boundaries to isolate concerns.

*   **CLI:** Responsible for argument parsing and triggering the Engine. Depends on Engine.
*   **Engine:** Orchestrates the flow. Depends on Bundle Loader, OPA Evaluation, and Evidence.
*   **Bundle Loader:** Strictly responsible for IO, decompression, and manifest validation. Has no knowledge of policy logic.
*   **OPA Evaluation:** Wraps the OPA SDK. Evaluates the loaded bundle. Has no knowledge of the CLI or file system IO.
*   **Evidence:** Appends records. Has no knowledge of OPA internals.
*   **Forbidden Dependencies:** The Evidence layer must never depend on the OPA Evaluation layer. The Bundle Loader must never depend on the Engine.

## 7. Determinism Model

The core invariant of Evidra is strict determinism. 
The function `f(input, bundle_revision, environment_label)` must yield the exact same decision, every time, on any machine. 
To achieve this, the `BundleArtifact` (the `.tar.gz`) is treated as entirely immutable. Once a revision is minted and packaged, its contents cannot be altered. The evaluation context contains no implicit state, external lookups, or time-based variability.

## 8. Evidence Model Extension

The Evidence Store schema is extended to capture the exact context of the bundle evaluation. Every evidence record must strictly record:

*   `bundle_revision`: Extracted directly from the bundle's `.manifest`.
*   `profile_name`: The logical name of the policy profile being executed.
*   `environment_label`: The opaque string provided at invocation.
*   `input_hash`: A cryptographic hash of the incoming plan or scenario.

Binding the `bundle_revision` to the evidence record is mandatory. Without it, the audit trail loses its deterministic link to the exact logic that produced the decision.

## 9. Migration Strategy

The transition from the current state to the OPA Bundle architecture requires a two-phase migration.

*   **Moving Inline Rules:** All fragmented Rego files must be restructured into the standard bundle directory layout.
*   **Moving Hardcoded Thresholds:** Any existing Rego policies containing inline thresholds or environment checks must be rewritten. The logic must be refactored to perform dictionary lookups against a unified data namespace, moving the hardcoded values into JSON data files within the bundle.

## 10. Acceptance Criteria

The implementation of this architecture will be validated against the following measurable checks:

*   The engine successfully loads a `.tar.gz` bundle and fails if the `.manifest` is missing.
*   Execution immediately terminates with an error if multiple bundles are supplied.
*   Static analysis of the Go codebase confirms zero conditional branches inspecting the environment label.
*   Static analysis of the Rego policies confirms zero string literals representing environment names.
*   The generated evidence record explicitly contains the `bundle_revision` parsed from the manifest.