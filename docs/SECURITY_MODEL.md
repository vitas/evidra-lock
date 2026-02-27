> Part of the Evidra OSS toolset by SameBits.

# Security Model

## Design Philosophy

Evidra is a pre-execution validation layer. The policy baseline (`ops-v0.1`) contains 23 rules focused exclusively on catastrophic failure prevention: production namespace deletion, world-open security groups, wildcard IAM policies, privileged container escape.

### What Evidra does

- Evaluates infrastructure changes against deterministic policy before execution.
- Returns structured decisions with rule IDs, reasons, and actionable hints.
- Records every decision (allow and deny) as tamper-evident evidence — hash-linked JSONL offline, Ed25519-signed records online.

### Non-goals

- **Not a CIS compliance scanner.** Does not implement full CIS benchmarks or aim for checkbox coverage.
- **Not a replacement for tfsec, trivy, or checkov.** Those tools perform deep static analysis across hundreds of rules. Evidra evaluates a small, curated set of catastrophic guardrails.
- **Not a runtime security tool.** No runtime API calls, no cloud provider connections, no live infrastructure inspection.
- **Not an enforcement gateway.** Evidra validates and records — it does not execute commands or manage infrastructure directly.

---

## Deterministic Evaluation

All policy evaluation is local and deterministic:

- Input is static configuration data (Terraform plan JSON, Kubernetes manifests, ArgoCD sync policies).
- No network calls during evaluation. No external dependencies at runtime.
- Given the same input and policy bundle, the same decision is produced every time.
- The OPA engine is embedded in the binary. Parameters are resolved from the bundle's `data.json` using a `by_env` fallback chain (environment-specific → default).
- Evaluation works in air-gapped environments.

In online mode, the API server runs the same OPA engine server-side. The evaluation is identical — only the transport and evidence format differ.

---

## Enforcement Model

Two modes are supported across all binaries (CLI, MCP, API):

- **Enforce** (default): Deny decisions block the action. The AI agent receives a structured denial and cannot proceed without addressing the policy violation.
- **Observe** (`--observe` / `EVIDRA_MODE=observe`): Policy is evaluated and recorded, but denials are downgraded to advisories. Useful for rollout and tuning.

In both modes, every decision is recorded to evidence. Observe mode does not skip logging.

---

## API Authentication

### Phase 0 — Static Key

A single API key is configured via `EVIDRA_API_KEY` (minimum 32 characters, 256 bits entropy when randomly generated). All requests to authenticated endpoints require `Authorization: Bearer <key>`.

**Timing attack mitigation:**
- Key comparison uses `crypto/subtle.ConstantTimeCompare` — never `==`.
- On auth failure: 50–100ms random jitter (from `crypto/rand`) before returning 401. This prevents timing side-channels from revealing partial key matches.

**Authenticated endpoints:** `POST /v1/validate`.
**Public endpoints:** `GET /v1/evidence/pubkey`, `GET /healthz`.

### Phase 1+ — Dynamic Keys

API keys are generated with 256 bits entropy from `crypto/rand`, stored as SHA-256 hashes. Plaintext is returned once at creation (with `Cache-Control: no-store`) and never persisted. No salt is needed — the key has sufficient entropy.

---

## Evidence Integrity

### Offline Evidence (CLI / MCP)

The evidence store at `~/.evidra/evidence` is append-only JSONL with hash-linked records:

- Each record includes `previous_hash` (linking to the prior record) and a self-verifying `hash`.
- The hash covers the canonical JSON representation of the record (excluding the `hash` field itself).
- Tampering with any record breaks the hash chain, detectable via `evidra evidence verify`.
- If evidence cannot be written, the validation pipeline returns an error — the caller cannot silently bypass logging.

### Online Evidence (API)

The API server signs every evidence record with Ed25519 and returns it in the response body. Evidence is never stored server-side — the client owns storage.

**Signing payload format:** Deterministic text, not JSON. Version prefix `evidra.v1\n`, then `key=value\n` fields in fixed declaration order (22 fields). List fields use length-prefixed encoding: `reasons=19:namespace.forbidden,14:image.unsigned`. This format is immune to JSON serialization differences across languages.

**Signature coverage:** The signature covers the `signing_payload` text field, not the full JSON record. Both the payload and base64-encoded signature are included in the response.

**Verification:** Anyone with the public key (from `GET /v1/evidence/pubkey`) can verify evidence offline — no server contact needed:
1. Decode the base64 signature.
2. Verify with `ed25519.Verify(pubkey, signingPayload, signature)`.

**`input_hash`:** SHA-256 of the marshaled invocation JSON. Opaque and server-internal — clients must not attempt to reproduce it.

Evidence records contain:

- Actor identity and origin (human, agent, system; cli, mcp, api).
- Tool, operation, and target parameters.
- Full policy decision (allow/deny, risk level, rule IDs, reasons, hints).
- Timestamps, event IDs, server ID, tenant ID, environment.
- Policy reference and bundle revision.

---

## Signing Key Management

The Ed25519 signing key is loaded at startup and never written to disk.

**Key resolution order:**
1. `EVIDRA_SIGNING_KEY` — base64-encoded private key (seed or full 64-byte key).
2. `EVIDRA_SIGNING_KEY_PATH` — path to PEM file (PKCS8 Ed25519 private key).
3. `EVIDRA_ENV=development` — ephemeral in-memory key generated from `crypto/rand`. Logs warning: *"using ephemeral signing key — evidence will not survive restart"*.
4. Any other `EVIDRA_ENV` (including unset / `production`) — `log.Fatal` + `os.Exit(1)`. Server refuses to start without a signing key.

The server uses `crypto/ed25519` from the Go standard library. No external cryptography dependencies.

---

## Input Validation

**Newline rejection:** Fields that appear verbatim in the signing payload (`actor.type`, `actor.id`, `actor.origin`, `tool`, `operation`, `environment`) are rejected if they contain `\n` or `\r`. Newlines would break the `key=value\n` payload format and could enable injection attacks. Returns 400 Bad Request.

**Request body limit:** 1MB maximum, enforced by `http.MaxBytesReader` middleware. Applied before JSON parsing.

**Structure validation:** `invocation.ValidateStructure()` checks required fields, actor type enum (`human`, `agent`, `system`), and origin enum (`mcp`, `cli`, `api`).

---

## Logging and Redaction

Evidra uses `log/slog` (structured JSON) for the API server and `log` for CLI/MCP.

**Never logged:**
- API key plaintext or signing key material.
- Full request bodies or evidence records.
- Any secret or credential value.

**Safe to log:**
- API key prefix (first 12 characters) for correlation.
- `tenant_id`, `event_id`, `tool`, `operation`.
- `decision.allow`, `risk_level`, `rule_ids`.
- Request method, path, status code, duration.
- Auth failure metadata (method, path, remote address — not the token).

---

## Known Limitations

- **Bypass risk:** Any execution path that skips `pkg/validate` / `evidra-mcp` / the API is not covered. If an agent can execute commands without calling the `validate` tool, Evidra provides no protection for that path.
- **Host-level access:** An adversary with root access to the host can rewrite the local evidence store directly. Mitigate by exporting evidence to an external system or using online mode where evidence is Ed25519-signed.
- **Static analysis only:** Evidra evaluates declared configuration, not runtime state. A Terraform plan that passes policy may still produce unexpected results due to provider behavior.
- **No rate limiting (Phase 0):** The static-key API has no request rate limiting. Rely on network-level controls (firewall, reverse proxy) for Phase 0.
- **Single static key (Phase 0):** All clients share one API key. No per-client audit trail until Phase 1 introduces dynamic key issuance.

---

## Recommended Deployment

- Run `evidra-mcp` in an isolated runtime with network controls so only trusted clients can submit tool invocations.
- Restrict agent shells so they cannot bypass the MCP server or the offline `evidra validate` CLI.
- Place the API server behind a reverse proxy (Traefik, nginx) with TLS termination. Do not expose the API without TLS.
- Use a persistent Ed25519 signing key in production (`EVIDRA_SIGNING_KEY` or `EVIDRA_SIGNING_KEY_PATH`). Ephemeral keys are for development only.
- Export evidence segments to a hardened store for long-term auditing: `evidra evidence export`.
- Start with `--observe` mode to validate policy against real workloads before switching to enforce.
