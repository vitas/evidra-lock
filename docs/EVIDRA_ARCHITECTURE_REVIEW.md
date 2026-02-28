# Evidra v0.1.16 — Expert Architecture Review

**Reviewer perspective:** Systems architect, security-focused, pre-release readiness  
**Scope:** Full source review — Go packages, OPA policy, MCP integration, API, evidence chain, Docker, documentation  
**Codebase stats:** ~16,200 LOC Go (95 files), ~8,750 LOC tests (39 files), ~54% test coverage by file count

---

## Executive Summary

Evidra is a well-architected, narrowly-scoped pre-execution policy gate for AI-driven infrastructure operations. The design philosophy — fail-closed, deterministic, no-LLM-in-the-loop — is sound and clearly communicated. The codebase is clean, idiomatic Go with minimal dependencies and a coherent layering model.

**Release readiness: Near-ready with caveats.** The core validation pipeline, evidence chain, and MCP integration are solid. The issues below range from "fix before release" (critical) to "address in v0.2" (strategic).

**Overall grade: B+** — Strong architecture, good security posture, needs hardening at the edges.

---

## What's Done Well

### 1. Design Clarity
The three-binary / shared-core architecture is clean. `pkg/` vs `internal/` separation is textbook correct — public API surface in `pkg/`, server internals in `internal/`. Zero coupling between the adapters repo and the main repo is a good boundary decision.

### 2. Fail-Closed Default
The `deny_insufficient_context` rule is the strongest design decision in the project. Unknown tools, missing payloads, and truncated context all produce denials. This is the correct default for a safety-critical system.

### 3. Security Fundamentals
- `crypto/subtle.ConstantTimeCompare` for key comparison
- 50–100ms jitter from `crypto/rand` on auth failure (timing attack mitigation)
- Ed25519 signing with deterministic text payload (not JSON — immune to serialization differences)
- Newline injection prevention in signing payload fields
- 1MB body limit middleware
- Distroless container images, nonroot UID
- API keys stored as SHA-256 hashes, plaintext returned once

### 4. Evidence Chain Integrity
The hash-linked JSONL with segmented storage is well-implemented. Evidence write failure propagates as an error — callers cannot silently bypass logging. The separation between offline (hash-chain) and online (Ed25519 signed) evidence models is appropriate.

### 5. OPA Integration
The bundle-based approach with `go:embed` is the right call. Embedded policy means zero-config works out of the box. The `by_env` parameter resolution chain is a pragmatic design for environment-specific overrides.

### 6. Hybrid Mode
The online/offline/fallback resolution is well thought out — mode is resolved at startup (no I/O), reachability tested at call time. The fallback classification (auth errors → fail hard, transport errors → fallback if configured) is correct.

---

## Critical Issues (Fix Before Release)

### C1. OPA Engine Reinitialized Per Evaluation

In `pkg/validate/EvaluateScenario()`, a new `runtime.Evaluator` (and therefore a new OPA `PreparedEvalQuery`) is created on every call. OPA query compilation is expensive — this adds 50–200ms of latency per validation in the CLI path. The MCP server path goes through the same `validate.EvaluateInvocation()` call.

**Impact:** Noticeable latency on every validation. In a CI pipeline evaluating dozens of scenarios, this compounds to seconds.

**Recommendation:** Cache the `runtime.Evaluator` instance. The policy bundle is immutable for the lifetime of the process. Create the evaluator once at startup and pass it through (or use a package-level singleton with sync.Once).

### C2. Evidence Event ID Is Not Globally Unique

```go
evidenceID = fmt.Sprintf("evt-%d", time.Now().UTC().UnixNano())
```

Nanosecond timestamps are not unique under concurrent access. Two goroutines or rapid sequential calls can produce the same ID. ULID is already a dependency (used in `internal/store`).

**Recommendation:** Use `ulid.Make().String()` prefixed with `evt-`, or at minimum add a random suffix.

### C3. `canonicalEvidenceRecord` Drift Risk Between Two `evidence` Packages

There are two separate `EvidenceRecord` types:
- `pkg/evidence/types.go` — local store (hash-chained)
- `internal/evidence/types.go` — API server (Ed25519 signed)

These have different field sets, different JSON tags, and different hash/signing semantics. The `canonicalEvidenceRecord` in `pkg/evidence` includes `BundleRevision`, `ProfileName`, `EnvironmentLabel`, `InputHash`, `Source` — but the canonical hash computation only covers a subset.

**Risk:** If someone adds a field to `EvidenceRecord` but forgets to add it to `canonicalEvidenceRecord`, that field isn't covered by the hash chain, enabling undetected tampering.

**Recommendation:** Add a compile-time assertion or test that verifies field parity. Alternatively, switch to an explicit allowlist in `ComputeHash` rather than a shadow struct.

### C4. Migration Runner Has No Idempotency Tracking

```go
func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
    // Executes ALL .sql files on every startup
```

There's no `schema_migrations` table. Every SQL file runs on every server restart. The DDL statements use `IF NOT EXISTS`, making them idempotent by accident — but any future migration that does `ALTER TABLE` or `INSERT` will break on restart.

**Recommendation:** Add a simple migration tracking table (`CREATE TABLE IF NOT EXISTS schema_migrations (filename TEXT PRIMARY KEY, applied_at TIMESTAMPTZ)`). Check before executing each file.

---

## Systemic Risks (Document Now, Address Strategically)

These are not bugs. They are architectural boundary conditions that should be explicitly stated in `SECURITY_MODEL.md` so users understand what Evidra does and does not guarantee.

### S1. Single-Process Trust Assumption

The entire security model rests on one premise: **the Evidra process is uncompromised**. Evidence is stored locally, policy is loaded from an embedded bundle, the signing key lives in process memory, and the migration runner can alter the database schema. There is no runtime attestation.

If an attacker gains access to the host and can replace the binary, swap the policy bundle, or modify environment variables, Evidra will not detect this. The hash chain and Ed25519 signatures protect against post-hoc tampering of evidence files, but not against a compromised process producing fraudulent evidence in real time.

**This is normal scope for v0.1.** But it must be stated explicitly.

#### Blast Radius by Compromise Level

The impact varies dramatically depending on what the attacker obtains:

| Compromise | Impact | Can Spoof Decisions | Can Rewrite History | Can Exfiltrate Secrets |
|------------|--------|---------------------|---------------------|----------------------|
| **API key leak** | Limited — can submit crafted invocations and receive allow/deny decisions. Can generate fraudulent evidence records via API. | Yes (new records only) | No (signed records are immutable once issued) | No |
| **Database access** | Key metadata exposure — tenant IDs, key hashes, usage timestamps. Cannot recover plaintext API keys (stored as SHA-256). | No (no signing key) | No | Partial (metadata only) |
| **Evidence FS access** | Can read all local evidence. Can truncate and rewrite the hash chain (rewind attack, see S2). Cannot forge Ed25519-signed API evidence. | No | Yes (local chain only) | Yes (evidence content) |
| **Host compromise** | Full trust collapse — replace binary, swap policy bundle, extract signing key from process memory, rewrite evidence, forge future decisions. | Yes | Yes | Yes |

This table should be included in `SECURITY_MODEL.md`. It helps users map Evidra's guarantees to their specific threat model.

#### Fundamental Design Classification

**Evidra is a preventive control, not a post-incident forensic system.** It prevents dangerous operations from executing. It does not investigate breaches after the fact. The evidence chain supports audit and accountability, but it is not designed for adversarial forensic analysis against a compromised host.

This distinction matters: preventive controls are evaluated by their false-negative rate and bypass resistance. Forensic systems are evaluated by evidence preservation guarantees under adversarial conditions. Evidra optimizes for the former.

#### Recommended Additions to `SECURITY_MODEL.md`

**Runtime Trust Boundary section:**
- Evidra assumes host integrity.
- No binary integrity validation (no secure boot, no attestation).
- Policy bundle integrity is guaranteed at build time (`go:embed`), not at runtime.
- Evidence integrity assumes an uncompromised writer process.
- Signing key confidentiality depends on OS-level process isolation.

**Explicit Non-Goals (v0.1) section:**
- Host compromise detection
- Runtime binary integrity verification
- Distributed consensus on evidence
- Byzantine fault tolerance
- Multi-region consistency
- Post-incident forensic chain-of-custody guarantees

Stating non-goals explicitly is one of the highest-leverage things a security document can do. It preempts "why don't you have X?" questions and signals architectural maturity to evaluators.

### S2. Evidence Chain: Rewind Attack (Truncation + Rewrite)

The hash chain protects against **modification** of existing records — changing a record breaks the chain. But it does not protect against **truncation**: an attacker with filesystem access can delete the last N records, recompute the chain from the new tail, and rewrite the manifest. `evidra evidence verify` will report the shortened chain as valid.

This is a known limitation of any purely local hash chain without an external anchor.

**Production-grade mitigation (not required for v0.1, but should be on the roadmap):**

Periodically publish the hash of the latest sealed segment to an external append-only store — S3 with Object Lock, a Git commit, an external audit service, or even a simple `anchor.json` file on a separate system.

Consider adding:
```
evidra evidence anchor export    # outputs {segment, hash, timestamp}
evidra evidence anchor verify    # compares local chain against external anchor
```

This closes the gap between "tamper-evident" (current) and "tamper-proof" (with anchoring). The distinction matters for compliance and audit scenarios.

### S3. MCP as an Escalation Vector: Input Depth and Complexity

The MCP server reads JSON from stdin, trusts the structure of incoming `ToolInvocation`, and passes it to OPA. The API has a 1MB body limit, but the MCP stdio path has no equivalent guard. There are no limits on:

- JSON nesting depth (deeply nested `params.payload` maps)
- Map fan-out (a `payload` with thousands of keys)
- Recursive or self-referential structures (Go's `encoding/json` handles this, but OPA's evaluation time scales with input size)

A malformed or adversarial invocation from a compromised AI host could trigger expensive OPA evaluation or excessive memory allocation.

**Recommendations:**
- Add a max JSON depth guard (e.g., reject inputs with nesting > 32 levels).
- Add a max map size limit on `params` and `payload` (e.g., 1000 keys).
- Pass an OPA evaluation context with a timeout (e.g., 5 seconds). Currently `context.Background()` is used, meaning evaluation runs until completion regardless of duration.

### Threat Model Overview

```
                    ┌─────────────────────────────────────────┐
                    │          AI Agent / CI Pipeline          │
                    │  (Claude Code, Cursor, GitHub Actions)   │
                    └──────────────┬──────────────────────────┘
                                   │
              Trust Boundary 1: Agent ↔ Evidra
              (MCP stdio / HTTP API)
                                   │
                    ┌──────────────▼──────────────────────────┐
                    │         Evidra Process                   │
                    │  ┌─────────────────────────────────┐    │
                    │  │  Input Validation & Canonicalize │    │
                    │  └──────────────┬──────────────────┘    │
                    │  ┌──────────────▼──────────────────┐    │
                    │  │  OPA Engine (embedded bundle)    │    │
                    │  └──────────────┬──────────────────┘    │
                    │  ┌──────────────▼──────────────────┐    │
                    │  │  Evidence Writer / Signer        │    │
                    │  └──────────────┬──────────────────┘    │
                    └──────────────┬──┴──────────────────────┘
                                   │  │
              Trust Boundary 2: Evidra ↔ Host OS
              (FS access, process memory, env vars)
                                   │  │
                    ┌──────────────▼──┼──────────────────────┐
                    │  Local Evidence Store   │  PostgreSQL   │
                    │  (~/.evidra/evidence)   │  (Phase 1+)   │
                    └────────────────────────┴───────────────┘
                                   │
              Trust Boundary 3: Local ↔ External (future)
              (Evidence anchoring, key escrow)
                                   │
                    ┌──────────────▼──────────────────────────┐
                    │  External Anchor (S3, Git, audit svc)   │
                    │  (not implemented — v1.0 roadmap)        │
                    └─────────────────────────────────────────┘
```

**What each boundary protects:**
- **Boundary 1:** Input validation, structure checks, policy evaluation. Compromised agent can submit crafted inputs but cannot bypass deny decisions (enforcement is server-side).
- **Boundary 2:** Assumed intact in v0.1. If breached, attacker has full control. Mitigate with OS hardening, container isolation, minimal privileges.
- **Boundary 3:** Not yet implemented. Required for evidence integrity guarantees against host compromise.

---

## Important Issues (Fix Before v0.2)

### I1. `isValidationError` Classifies Errors via String Matching

```go
func isValidationError(err error) bool {
    msg := err.Error()
    return strings.Contains(msg, "is required") ||
        strings.Contains(msg, "must be") ||
        strings.Contains(msg, "unknown")
}
```

This is fragile in two ways:
- Any change to error text in `ValidateStructure()` silently changes HTTP status codes and client-visible behavior.
- The contract between `pkg/invocation` and `internal/api` cannot be stably tested — there's no type or sentinel to assert against.
- If an OPA policy evaluation happens to produce an error message containing "is required", it'll be misclassified as a 400 instead of 500.

**Recommendation:** In `pkg/invocation`, introduce a typed error: `type ValidationError struct { Field, Code string }` (or at minimum sentinel errors per category). In the API handler, check with `errors.As(&validationErr)` / `errors.Is()` instead of substring matching. This makes the contract explicit and testable.

### I2. `buildCanonicalAction` Silently Drops Unknown Params

In `validate_handler.go`, `buildCanonicalAction(&inv)` rebuilds `inv.Params` from scratch, keeping only `action` and `scenario_id`. This creates two problems:

1. **Input mutation:** The original `inv.Params` is replaced in-place. If any downstream code (logging, evidence, middleware) references the original params, the data is gone.

2. **Silent field loss:** If a client sends additional parameters (e.g., future fields, custom metadata), they are silently discarded. No error, no warning, no trace. This makes debugging harder and breaks forward compatibility — a v0.2 client sending a new field to a v0.1 server gets no feedback that the field was ignored.

**Recommendation (pick one or both):**
- Preserve unrecognized fields in a `params._extra` or `params.meta` sub-map so they survive through to evidence records without affecting policy evaluation.
- Alternatively, explicitly document: *"Any param key other than target/payload/risk_tags/scenario_id is silently dropped by the API."* — and consider returning a warning in the response when unknown fields are present.
- Either way, clone `inv` before mutation to avoid corrupting the original.

### I3. No Rate Limiting on `POST /v1/keys`

The CHANGELOG mentions "Rate-limited: 3 keys/hour/IP" but there's no rate limiting implementation in the code. The `handleKeys` handler has no rate limit middleware. The `InviteSecret` gate provides some protection, but without rate limiting, an attacker who discovers the invite secret can create unlimited tenants.

**Recommendation:** Implement the documented rate limiting before advertising it. A simple in-memory token bucket per IP would suffice for Phase 1.

### I4. `TouchKey` Goroutine Leak on Shutdown

```go
func (s *KeyStore) TouchKey(keyID string) {
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        // ...
    }()
}
```

Fire-and-forget goroutines with `context.Background()` won't be cancelled during shutdown. If the database pool closes before the goroutine completes, it'll produce errors.

**Recommendation:** Use a buffered channel or `errgroup` to track pending touch operations and drain them during graceful shutdown.

### I5. Evidence Store: Scaling and Operability at Volume

Segmentation is in place (5MB segments, manifest, flock) — this is already good. But two scenarios deserve attention:

**Concurrent writers:** MCP server + CLI running in parallel (e.g., CI pipeline validating while an agent is actively using the MCP server). The `flock`-based locking is correct for single-host, but under high contention the 2-second timeout (`defaultLockTimeoutMS`) may cause spurious `evidence_store_busy` errors. Consider making the retry backoff configurable or documenting expected concurrency limits.

**Large-scale verify/export:** `ValidateChainAtPath` and `ReadAllAtPath` both stream through every record from the first segment. On a store with hundreds of MB or GB of evidence (realistic for a team running dozens of agents), `evidra evidence verify` becomes a multi-minute operation with no progress feedback.

**Recommendations:**
- Add progress output for long-running CLI operations (`verify`, `export`): record count, segment progress, elapsed time.
- Add `evidra evidence verify --last N` or `--from-cursor` mode for quick health-checks without full chain traversal. The forwarder cursor infrastructure (`ForwarderState`) already exists — reuse it for incremental verification.
- Consider an optional event_id → segment:offset index file for O(1) lookups (currently `FindByEventID` is O(N) linear scan).

### I6. API Hardening: Content-Type, Timeouts, Rate Limiting

The existing security surface is solid (body limit, anti-newline injection, constant-time compare + jitter). Three gaps remain:

**Strict Content-Type enforcement:** `POST /v1/validate` accepts any Content-Type and attempts JSON decode. Adding a `Content-Type: application/json` check reduces noise from scanners and accidental browser form submissions.

**HTTP timeouts:** The server sets `ReadHeaderTimeout: 10s` and `IdleTimeout: 60s` — good. But `ReadTimeout` and `WriteTimeout` are unset (zero = unlimited). A slow client can hold a connection open indefinitely during body read or response write. For a public API, set `ReadTimeout: 30s` and `WriteTimeout: 30s`.

**Rate limiting:** The security model document explicitly states *"No rate limiting (Phase 0)"* — this honesty is appreciated, but the gap should be closed. Options:
- Minimal built-in rate limiter (token bucket per IP, stdlib `sync` + `time` — no external dependency needed).
- Or prominently document: *"Phase 0 MUST be deployed behind a reverse proxy with rate limiting."* with a concrete nginx/Traefik config example.

The `POST /v1/keys` endpoint is especially sensitive — no rate limiting there means unlimited tenant creation (documented as "3 keys/hour/IP" but not implemented).

---

## Moderate Issues

### M1. `copyMap` Is Shallow Copy
`copyMap` in `validate.go` copies one level deep. Nested maps (common in Terraform payloads) share references with the original. Mutations to nested objects in the copy will affect the original.

### M2. No CORS Headers on API
The API server has no CORS configuration. The embedded UI (`uiembed_embed.go`) serves from the same origin, which works — but if the UI is ever served from a different domain, or third-party tools call the API from browsers, this will silently fail.

### M3. Logging Configuration in MCP Binary
The MCP binary uses `log.New(stderr, ...)` (standard library logger) while the API binary uses `slog` (structured JSON). For operational consistency, both should use the same logging framework.

### M4. `splitStatements` SQL Parser Is Naive
The semicolon-splitting migration parser will break on SQL containing semicolons in string literals (e.g., `INSERT INTO ... VALUES ('contains;semicolon')`). Currently safe because all migrations are DDL, but will bite when data migrations are added.

### M5. No Request Timeout on Validate Handler
The API's `POST /v1/validate` handler has no per-request timeout. OPA evaluation is normally fast, but a pathological input could cause the engine to compute longer than expected. `ReadHeaderTimeout` protects the read phase but not the handler.

---

## OPA Policy Review

The policy bundle is well-structured. Observations:

### P1. Fail-Open in OPA Is Deeper Than `default allow := true`

```rego
default allow := true
```

The surface issue is that OPA's default is `allow: true`. But the problem goes further. If the policy bundle fails to load, `data.json` is corrupted, or a rule file doesn't compile, the OPA evaluator can: return a nil result set, return `allow: true` with no hits, or return a result that doesn't match the expected structure.

The Go layer in `pkg/policy/policy.go` does check for `len(results) == 0` and validates the `allow`, `reason`, and `risk_level` fields — this is good. But it does **not** verify that the result came from a fully loaded policy bundle. Specifically:

- No check that `hits` or `rule_ids` is non-nil (an empty hits list on a destructive operation may indicate rules didn't fire, not that the operation is safe).
- No check that the profile name matches the expected bundle (`ops` vs `baseline`).
- No canary rule to confirm the decision aggregator actually loaded.

**Recommendation (defense-in-depth):** After `Evaluate()`, add structural assertions:
1. If the operation is destructive and `allow: true` but `hits` is empty, verify a canary rule fired (add a `sys.bundle_loaded` rule that always produces a hit).
2. Verify that `decision.ProfileName` matches the expected profile from the bundle manifest.
3. If the result structure is invalid or incomplete, fail hard (deny) rather than falling through to `default allow := true`.

**Decision schema versioning:** There is an additional subtle risk. If the Rego decision object gains new fields in a future policy bundle, the Go layer will silently ignore them (Go's `json.Unmarshal` drops unknown fields by default). Conversely, if a field is removed from the Rego output, the Go layer may operate on zero-values without warning.

The decision schema between Go and Rego is a contract. It should be versioned. A breaking change in the decision object structure must increment a decision schema version (e.g., `decision.schema_version: 1`), and the Go layer should reject decisions with an unexpected schema version. This is the kind of detail that distinguishes a tool from a product.

### P2. Policy Tests Are Comprehensive
28 test files covering deny rules, warn rules, environment-specific behavior, resolve_param, and decision contract. This is good coverage for the OPA layer.

### P3. Add Policy Contract Golden Tests to CI

The `examples/` directory contains scenario fixtures, but they aren't wired as automated regression tests against expected decisions. Adding golden tests — each fixture paired with an expected `{allow, rule_ids, risk_level}` — would provide:

- **Regression protection** when editing `.rego` files (a rule refactor that accidentally changes behavior is caught immediately).
- **Pre-release confidence** (run the golden suite as a CI gate before tagging a release).
- **Living documentation** (the fixture set *is* the policy contract, not just examples).

Implementation: a simple Go test in `pkg/validate` that iterates `examples/demo/*.json`, evaluates each, and asserts against a `.expected.json` sidecar file.

### P4. Rule Authoring Needs a Checklist

The extension points exist (`docs/POLICY_CATALOG.md`, `docs/CONTRIBUTING.md`), but there's no step-by-step guide for adding a custom rule. A "Rule Authoring Checklist" would lower the barrier for contributors:

1. Create `deny_<name>.rego` in `evidra/policy/rules/` with consistent package declaration.
2. Add hint entries in `evidra/data/rule_hints/data.json`.
3. Add entry to `docs/POLICY_CATALOG.md` (ID, severity, description, remediation).
4. Add test file `tests/deny_<name>_test.rego` with at least one allow and one deny case.
5. Add a scenario fixture in `examples/` with the expected decision.
6. Gate on `profile_includes_ops` if the rule is ops-layer (not baseline).

A template `.rego` file with these placeholders would make it copy-paste-ready.

---

## Documentation Quality

**Strong.** The README is well-paced, the 30-second demo is compelling, and the architecture doc is genuinely useful. The security model document is thorough and honest about limitations.

Minor gaps:
- No API reference (OpenAPI/Swagger). The architecture doc lists endpoints but doesn't specify request/response schemas.
- `CONTRIBUTING.md` could mention how to add custom OPA rules (the primary extension point).
- No `RELEASE.md` or goreleaser config in the repo (mentioned in CONTRIBUTING but absent).

---

## Docker & Operations

**Dockerfile quality is high.** Multi-stage builds, distroless base, nonroot, CGO_ENABLED=0, `-trimpath -ldflags="-s -w"`. The `docker-compose.yml` has a hardcoded default API key — fine for dev, but should include a comment warning against production use.

Missing: no Kubernetes manifests, no Helm chart, no Terraform module for deployment. For an infrastructure-focused tool, this is a notable omission.

---

## Dependency Assessment

Minimal and appropriate:
- `github.com/open-policy-agent/opa` — core business logic, justified
- `github.com/modelcontextprotocol/go-sdk` — MCP integration, justified
- `github.com/oklog/ulid/v2` — ID generation, lightweight
- `github.com/jackc/pgx/v5` — Postgres driver, industry standard
- `go.yaml.in/yaml/v3` — scenario loading

No unnecessary frameworks. No HTTP router library (uses `net/http` stdlib). No ORM. This is commendable restraint.

---

## Summary of Recommendations by Priority

| Priority | Issue | Effort |
|----------|-------|--------|
| **Fix before release** | C1: Cache OPA evaluator | Small |
| **Fix before release** | C2: Use ULID for event IDs | Trivial |
| **Fix before release** | C3: Canonical record drift test | Small |
| **Fix before release** | C4: Add migration tracking | Small |
| **Document before release** | S1: Single-process trust boundary in SECURITY_MODEL.md | Small |
| **Document before release** | S2: Evidence rewind attack limitation | Small |
| v0.2 | S3: MCP input depth/complexity guards + OPA timeout | Medium |
| v0.2 | I1: Typed validation errors (not string matching) | Small |
| v0.2 | I2: Don't silently drop unknown params; don't mutate input | Small |
| v0.2 | I3: Implement rate limiting on /v1/keys | Medium |
| v0.2 | I4: Drain TouchKey goroutines on shutdown | Small |
| v0.2 | I5: Evidence scaling — progress, incremental verify, index | Medium |
| v0.2 | I6: API hardening — Content-Type, timeouts, rate limiting | Medium |
| v0.2 | P1: OPA fail-open defense — canary rule + result validation | Medium |
| v0.2 | P3: Policy contract golden tests in CI | Medium |
| v0.2 | P4: Rule authoring checklist + template | Small |
| v1.0 | Evidence anchoring (external append-only store) | Large |
| Backlog | M1–M5, API docs, Helm chart | Various |

---

## Architectural Maturity Assessment

| Dimension | Level | Notes |
|-----------|-------|-------|
| Code Quality | High | Idiomatic Go, minimal deps, clean layering |
| Security Posture | Above Average | Constant-time compare, Ed25519, anti-injection, jitter |
| Runtime Hardening | Moderate | Body limits yes; input depth, OPA timeout, Content-Type enforcement missing |
| Scalability | Moderate | Segmented store good; O(N) lookups, per-call OPA init are bottlenecks |
| Multi-Tenant Readiness | Early | Phase 1 key store works; no per-tenant rate limiting or isolation |
| Evidence Integrity (local) | Strong | Hash chain, flock, segment rotation, tamper detection |
| Evidence Integrity (distributed) | Not Yet | No external anchoring, no rewind attack protection |
| Policy Extensibility | Good | Bundle structure, by_env params, rule hints; needs authoring guide |
| Documentation | Strong | Honest security model, clear architecture doc, good README |

---

## A Note on Positioning

Evidra is not "AI safety" in the broad sense. It is something more specific and more defensible:

**Deterministic Pre-Execution Guardrail with Evidence-Backed Audit Trail.**

The messaging could be sharper. Consider framing around:
- *"Agent Execution Firewall"* — immediately communicates the function.
- *"Policy Gate with Cryptographic Audit"* — emphasizes the evidence chain.
- *"Infrastructure Kill-Switch"* — the README already uses this, and it's the strongest hook.

The current README does this well. The key is to avoid positioning alongside compliance scanners (tfsec, Checkov) or runtime security tools (Falco, OPA Gatekeeper). Evidra occupies a distinct pre-execution niche that doesn't exist in the current tooling landscape, and the messaging should protect that positioning.

---

## Closing Assessment

This is a serious, well-thought-out project. The problem space (AI agents executing infrastructure commands without guardrails) is real and growing. The approach — deterministic OPA policy, fail-closed, evidence chain — is architecturally sound.

The codebase reads like it was written by someone who understands both infrastructure operations and software engineering fundamentals. The security posture is above average for an early-stage OSS project. The `"Generated by AI"` comments in some files are transparent and appreciated — the code quality is consistent regardless of origin.

Evidra reduces the probability of catastrophic automated infrastructure mistakes. It does not eliminate risk — it narrows it deterministically.

The systemic risks identified (single-process trust, evidence rewind, MCP depth) are not blockers — they are the natural boundaries of a v0.1 local-first tool. What matters is that they are documented honestly so users can build appropriate threat models.

**For a v0.1 release**: fix C1–C4, document the systemic trust boundaries (S1–S3), and this is ready to ship.

**Maturity trajectory:**
- **v0.1** — Single-node trust, local evidence, embedded policy. *(current)*
- **v0.2** — Multi-tenant hardened, rate limiting, evidence anchoring, OPA canary.
- **v1.0** — Distributed safe, pluggable evidence backends, key rotation, external attestation.
