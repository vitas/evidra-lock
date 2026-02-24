# Architecture Guardrails: OPA Bundles

**Role:** Principal Software Architect  
**Status:** Approved  
**Topic:** Enforcement of Policy Architecture Invariants  

## 1. Architectural Invariants

The following principles are absolute and must not be violated under any circumstances.

*   **No Environment Branching in Go:** The Evidra execution engine must treat the environment identifier exclusively as an opaque string passed from input to the OPA context. There must be zero `if/switch` statements in the Go codebase that inspect the value of the environment label.
*   **No Environment Literals in Rego:** Rego policies must not contain hardcoded string literals to match environments (e.g., `input.environment == "production"`).
*   **All Thresholds in Data Namespace:** All numerical limits, allowlists, denylists, and environmental overrides must exist as JSON within the bundle's data namespace.
*   **Single-Bundle Execution Only:** The engine is restricted to loading exactly one `.tar.gz` bundle per execution lifecycle.

## 2. Prohibited Patterns

Code reviews and automated checks must reject the following anti-patterns:

*   **Hardcoded Thresholds:** Embedding limits directly in Rego logic (e.g., `count > 5`) instead of referencing the data namespace (`count > data.limits.max_instances`).
*   **Inline Environment Checks:** Writing Rego rules that change behavior based on hardcoded environment names rather than relying on the data namespace lookup mechanism.
*   **Silent Fallback Bundles:** Attempting to implement a "base" bundle that loads alongside a "specific" bundle. This violates the single-bundle invariant and destroys deterministic traceability.
*   **Multi-Bundle Creep:** Proposing features that allow directories of bundles or array inputs for bundle paths.

## 3. CI Enforcement Strategy

To prevent architectural regression, the Continuous Integration pipeline must enforce these guardrails mechanically.

*   **Linting Rules:** 
    *   Implement Go linters (e.g., `golangci-lint` with custom rules if necessary) to detect and block any conditional logic analyzing the `environment` variable.
    *   Utilize standard Rego linters (e.g., `Regal`) configured to flag inline string comparisons that resemble environment names.
*   **Review Checklist:** Every Pull Request modifying the Go execution engine or Rego policies must include a mandatory checklist confirming adherence to the data-driven environment model and single-bundle invariant.
*   **Manifest Validation:** The CI pipeline must strictly validate the presence and structure of the `.manifest` file in the source repository before allowing a build to proceed.
*   **Namespace Validation:** Automated tests must verify that all Rego rules rely on externalized data references rather than inline constants.

## 4. Drift Detection Model

To detect accidental architectural violations that might slip past initial CI checks:

*   **Static Code Analysis:** Regularly run AST (Abstract Syntax Tree) analysis on the Go engine to guarantee the isolation of the `Bundle Loader` from the `OPA Evaluation` phase.
*   **Data Namespace Audit:** Periodically audit the bundle source tree to ensure no Rego file has introduced new logic that circumvents the data files. If the ratio of Rego logic to JSON data complexity skews heavily towards Rego over time, it indicates drift away from the data-driven invariant.
*   **Evidence Review:** Periodically sample the Evidence Store to confirm that every recorded decision possesses a valid, non-empty `bundle_revision` matching a known, published artifact.