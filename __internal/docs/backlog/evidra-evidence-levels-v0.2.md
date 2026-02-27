# Evidra — Evidence Levels Model v0.2

This document defines configurable evidence depth for Evidra.

Goal: preserve accountability while allowing different privacy and
storage trade-offs.

The system MUST remain append-only and hash-chained at all levels.

---

## Current State

The existing `EvidenceRecord` stores `Params map[string]interface{}` —
the full payload including target, payload content, risk_tags, and
scenario metadata. This is effectively "full" mode without redaction.

The levels model does not add data — it restricts what gets written.

---

## 1. Design Goals

1. Preserve integrity (tamper-evident chain) at all levels.
2. Default mode is safe for production — no raw secrets stored.
3. Enable forensic verification without storing raw content.
4. Full mode available for incident investigation (opt-in, with redaction).
5. Single reduction point before store append — no logic duplication.

---

## 2. Evidence Levels

Two levels. Not three.

The difference between "decision only" and "decision + digests" is ~100
bytes per record with zero privacy risk. There is no realistic reason to
store a decision without at least being able to verify what input produced
it. Therefore digests are always included in the default.

### Level: default

Purpose: production-safe accountability with forensic verifiability.

Stored fields:

```
Chain integrity:
  - event_id
  - timestamp
  - previous_hash
  - record_hash
  - evidence_level: "default"

Identity:
  - actor (type, id, origin)
  - source

Operation:
  - tool
  - operation
  - environment_label

Policy:
  - policy_ref
  - bundle_revision
  - profile_name
  - input_hash (sha256 of full canonical input — already exists)

Decision:
  - policy_decision.allow
  - policy_decision.risk_level
  - policy_decision.reason
  - policy_decision.reasons[]
  - policy_decision.rule_ids[]
  - policy_decision.hints[]
  - policy_decision.advisory
  - breakglass_used (derived from risk_tags)

Digests (new fields):
  - target_digest (sha256 of canonical JSON of params.target)
  - payload_digest (sha256 of canonical JSON of params.payload)
  - payload_shape (list of top-level keys in payload, e.g. ["namespace", "containers", "resource"])
```

NOT stored:

- Raw payload content
- Raw target content
- Risk tags (beyond breakglass flag)
- Execution stdout/stderr

What this enables:

- **"Who did what?"** — actor + tool + operation + timestamp
- **"Was it allowed?"** — policy_decision with full rule hits and reasons
- **"Can I verify the input?"** — present the original payload, compute sha256, compare to payload_digest
- **"Why was it denied?"** — reasons + hints
- **"What shape was the input?"** — payload_shape shows which fields were present (helps debug insufficient_context)

`payload_shape` is always included, not optional. It has zero privacy risk
(key names, not values) and is invaluable for debugging: "why deny?" →
"because payload contained only [namespace] without [containers]".

### Level: full (opt-in)

Purpose: incident investigation and compliance audit export.

Additional stored fields (on top of default):

```
Redacted content:
  - redacted_target (target with sensitive keys stripped)
  - redacted_payload (payload with sensitive keys stripped)
  - risk_tags[]

Execution (if available):
  - execution_result.status
  - execution_result.exit_code

Metadata:
  - evidence_level: "full"
  - environment metadata (if provided)
```

NOT stored even in full mode:

- Raw passwords, tokens, secrets, keys (redacted before write)
- execution_result.stdout / stderr (too large, not policy-relevant)

What this additionally enables:

- **"What exactly was evaluated?"** — see the actual payload (redacted)
- **"What changed?"** — compare redacted payloads between allow and subsequent deny
- **Compliance export** — hand the evidence chain to auditors with readable content

---

## 3. Redaction Rules (full mode only)

Redaction is applied before writing. It is key-name based and operates
recursively on nested structures.

### Sensitive key patterns

Default sensitive keys (matched case-insensitively, including nested keys):

```
password, passwd, secret, token, key, private_key,
authorization, api_key, apikey, access_key, secret_key,
credential, credentials, connection_string, dsn, bearer,
session, cookie, aws_secret_access_key, database_url,
client_secret, signing_key, encryption_key, ssh_key
```

This list is configurable via `evidence.redaction_keys` for organizations
with custom naming conventions.

### Redaction behavior

1. Known sensitive keys → value replaced with `"[REDACTED]"`
2. Strings longer than `max_field_bytes` → truncated with `"[TRUNCATED: 4096 of 12340 bytes]"`
3. Arrays longer than 50 entries → truncated with summary: `"[TRUNCATED: 50 of 200 entries]"`
4. Binary-looking values (base64) → replaced with `"[BINARY: sha256=abc..., 1234 bytes]"`
5. Structural shape preserved for readability

### Example

Input:
```json
{
    "image": "nginx:1.25",
    "namespace": "production",
    "env": [
        {"name": "DB_URL", "value": "postgres://user:secret@host/db"},
        {"name": "API_KEY", "value": "sk-abc123..."},
        {"name": "LOG_LEVEL", "value": "info"}
    ],
    "containers": [{"name": "web", "image": "nginx:1.25"}]
}
```

Stored (redacted):
```json
{
    "image": "nginx:1.25",
    "namespace": "production",
    "env": [
        {"name": "DB_URL", "value": "[REDACTED]"},
        {"name": "API_KEY", "value": "[REDACTED]"},
        {"name": "LOG_LEVEL", "value": "info"}
    ],
    "containers": [{"name": "web", "image": "nginx:1.25"}]
}
```

Note: redaction is best-effort, not a guarantee. If a secret is stored
in a field named `description`, it will not be redacted. The full mode
warning in configuration explicitly states this:

> "Redaction is key-name based. Review evidence output before enabling
> full mode in environments with sensitive data. Consider encryption-at-rest
> for additional protection."

---

## 4. Implementation Strategy

### Reduction function

```
func Reduce(record EvidenceRecord, level string) EvidenceRecord
```

Called once, after building the full record in memory but before
appending to the store.

```
Build full EvidenceRecord in memory (as today)
   ↓
Apply Reduce(record, level)
   ↓
Set record.evidence_level = level
   ↓
Compute record hash (on reduced record)
   ↓
Append to store
```

### Canonicalization Requirements

Digests (`target_digest`, `payload_digest`) are only useful if the same
logical input always produces the same hash. Go's `json.Marshal` does NOT
guarantee key ordering for `map[string]interface{}`. A custom canonical
serializer is required.

Requirements:

1. **Deterministic key sorting**: all object keys sorted lexicographically at every nesting level
2. **No whitespace**: compact output, no spaces or newlines
3. **Stable number formatting**: integers as integers (`42`, not `42.0`); floats via `strconv.FormatFloat(x, 'f', -1, 64)` — no scientific notation, minimal digits
4. **Nil vs empty**: `nil` map → `null`; empty map `{}` → `{}`; these MUST produce different hashes
5. **Empty arrays**: `nil` slice → `null`; empty slice `[]` → `[]`
6. **UTF-8**: all strings UTF-8 encoded, no BOM, no normalization beyond Go's default
7. **No trailing content**: no trailing newline or whitespace after the closing brace

Implementation note: the existing `scenarioHash()` in `pkg/validate` uses
`json.Marshal` which sorts keys for structs but NOT for `map[string]interface{}`.
For evidence digests, use a canonical JSON function that recursively sorts
map keys. Consider `github.com/cyberphone/json-canonicalization` (RFC 8785)
or a minimal in-house implementation (~50 LOC).

Test: the same payload constructed in two different orders must produce
identical `payload_digest`. Add a unit test that verifies this explicitly.

### Canonical version

Each record includes `canonical_version: "v1"` to track which
canonicalization algorithm produced the digests. This tracks the canonical
JSON algorithm, not the EvidenceRecord schema version. If the algorithm
changes in the future (e.g. adopting strict RFC 8785), the version bumps
to `"v2"`. Readers compare digests only against records with matching
canonical_version.

This is cheap to add now and avoids "digests stopped matching after upgrade"
surprises later.

### Nil and empty value handling

Explicit rules for edge cases:

| input | canonical form | sha256 of |
|---|---|---|
| `nil` map | `"null"` | `sha256("null")` |
| empty map `map[string]interface{}{}` | `"{}"` | `sha256("{}")` |
| `nil` slice | `"null"` | `sha256("null")` |
| empty slice `[]interface{}{}` | `"[]"` | `sha256("[]")` |

If `params.target` is nil → `target_digest = sha256("null")`.
If `params.payload` is nil → `payload_digest = sha256("null")`.

These MUST produce different digests from empty objects. A nil payload
(agent sent nothing) is semantically different from an empty payload
(agent sent `{}`), and the digests must reflect this.

### Digest Scope

| digest field | computed from | timing |
|---|---|---|
| `target_digest` | `params.target` (original, pre-redaction) | Before Reduce clears/redacts params |
| `payload_digest` | `params.payload` (original, pre-redaction) | Before Reduce clears/redacts params |
| `input_hash` | Full canonical input (already exists) | At evaluation time, before evidence |

Digests always reflect what was actually evaluated by policy, not the
redacted or reduced version. This is what makes forensic verification
work: hash the original payload, compare to `payload_digest`.

For **default**:
1. Compute and set `target_digest` = sha256(canonical(params.target))
2. Compute and set `payload_digest` = sha256(canonical(params.payload))
3. Compute and set `payload_shape` = sorted top-level keys of params.payload
   - Top-level keys ONLY — nested keys are not included
   - Computed from the ORIGINAL payload before any reduction or redaction
   - Example: `{"namespace":"x", "containers":[...], "resource":"pod"}` → `["containers", "namespace", "resource"]`
   - This is not a schema or full field coverage — it shows what the agent provided at the top level
4. Set `breakglass_used` = true if risk_tags contains "breakglass"
   - MUST be derived BEFORE params is cleared (default) or redacted (full)
   - `breakglass_used` is stored at both levels
   - `risk_tags` array is only stored in full mode
   - This ensures the breakglass signal survives reduction even when raw tags are stripped
5. Set `canonical_version` = "v1"
6. Clear `params` (remove raw content)
7. Clear `execution_result.stdout`, `execution_result.stderr`

For **full**:
1. Apply redaction to params.target → `redacted_target`
2. Apply redaction to params.payload → `redacted_payload`
3. Compute digests (same as default, from pre-redaction data)
4. Compute payload_shape (same as default)
5. Set breakglass_used
6. Replace params with redacted versions
7. Clear execution stdout/stderr

### Hash chain integrity

The hash is always computed on the **reduced** record. This means:
- Chain is valid regardless of level
- Changing level mid-chain is safe (records before and after have different structure but valid hashes)
- Each record includes `evidence_level` field so readers know how to interpret it

### Mixed-level chains

If a user changes level from `default` to `full` (or vice versa), the
chain continues without breaking. The `evidence_level` field in each
record tells the reader what fields to expect. Chain validation checks
`previous_hash` linkage, which works across levels since hashes are
computed on the final (reduced) record.

---

## 5. Configuration

### Environment variable

```
EVIDRA_EVIDENCE_LEVEL=default|full
```

### Config file (optional)

```yaml
evidence:
  level: default
  max_field_bytes: 4096          # full mode only: truncation threshold
  retention_days: 0              # 0 = infinite (user manages cleanup)
  redaction_keys: []             # additional sensitive key names
  redaction_mode: strict         # strict (default) | permissive
```

### Defaults

| setting | default | rationale |
|---|---|---|
| level | `default` | Safe, lightweight, forensic-verifiable |
| max_field_bytes | `4096` | Reasonable for most payloads |
| retention_days | `0` (infinite) | Compliance typically requires 90-365 days; safer to not auto-delete |
| redaction_mode | `strict` | Strip all matches; `permissive` only hashes them |

### Why retention_days defaults to 0

Evidence is an audit trail. Auto-deleting audit records is dangerous for
compliance. `retention_days: 30` (v0.1 suggestion) is too aggressive.
Users who need rotation should set it explicitly, understanding the
implications.

---

## 6. Pack Compatibility

### Pack 2 — DevOps Baseline (fail-closed + unknown tool guard)

| level | what's stored | use case |
|---|---|---|
| default | decision + actor + digests + payload_shape | "Who ran what, was it allowed, can I verify?" |
| full | above + redacted payload/target | "What exactly was the payload?" |

### Pack 3 — Ops Disaster Prevention (23 golden rules)

Same as Pack 2. Evidence levels are orthogonal to policy packs.

---

## 7. Product Positioning

Evidence is the third value pillar, not the first:

1. **Kill-switch** (Pack 2) → "agent won't destroy prod"
2. **Policy** (Pack 3) → "agent won't create dangerous configs"
3. **Evidence** → "you know who did what and what was approved"

Evidence sells itself after the first incident. Before that, it's a quiet
safety net that runs automatically.

### How evidence answers real questions

| question | what answers it |
|---|---|
| "Did the agent or the human run this?" | `actor.type` + `actor.origin` |
| "Was this action approved by policy?" | `policy_decision.allow` + `rule_ids` |
| "What policy version was active?" | `policy_ref` + `bundle_revision` |
| "Was breakglass used?" | `breakglass_used` |
| "Can I prove this payload was checked?" | `payload_digest` — hash the original, compare |
| "What fields were in the input?" | `payload_shape` |
| "What exactly was sent?" (full only) | `redacted_payload` |
| "When did this happen?" | `timestamp` + `event_id` |
| "Is the audit trail intact?" | `evidra evidence verify` — validates hash chain |

### One-liner for README

```
Every decision — allow and deny — is recorded to a tamper-evident local
evidence chain. You always know who did what, when, and what was approved.
```

---

## 8. New Fields Summary

Fields to add to `EvidenceRecord` struct:

```go
type EvidenceRecord struct {
    // ... existing fields ...

    // Evidence level control (new)
    EvidenceLevel    string   `json:"evidence_level"`              // "default" | "full"
    BreakglassUsed   bool     `json:"breakglass_used,omitempty"`
    CanonicalVersion string   `json:"canonical_version,omitempty"` // "v1" — tracks digest algorithm

    // Digests (new, always present)
    TargetDigest     string   `json:"target_digest,omitempty"`     // sha256 of canonical target
    PayloadDigest    string   `json:"payload_digest,omitempty"`    // sha256 of canonical payload
    PayloadShape     []string `json:"payload_shape,omitempty"`     // sorted top-level keys

    // Full mode only (new)
    RedactedTarget   map[string]interface{} `json:"redacted_target,omitempty"`
    RedactedPayload  map[string]interface{} `json:"redacted_payload,omitempty"`
}
```

The existing `Params` field behavior changes:
- **default level**: `Params` is cleared after digests are computed (set to nil)
- **full level**: `Params` is replaced with redacted versions

This is backward compatible — existing records without `evidence_level`
are treated as pre-levels (equivalent to current behavior where full
params are stored).

---

## 9. Migration

Existing evidence chains store full `Params`. After implementing levels:

- Old records: no `evidence_level` field → reader treats as "legacy full"
- New records: `evidence_level` present → reader uses it
- Hash chain continues unbroken — hashes are per-record, level change is safe
- No migration needed. Old data stays as-is.

**Legacy data warning:** records written before evidence levels were
implemented contain raw, unredacted params. Readers MUST NOT assume
redaction rules were applied to legacy records. If exporting evidence
for audit or sharing, legacy records should be post-processed through
the redaction function or flagged as "pre-redaction era". The absence
of `evidence_level` field is the indicator.

---

END OF DOCUMENT v0.2
