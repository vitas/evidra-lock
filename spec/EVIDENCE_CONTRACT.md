---

# Evidence Contract v0.1

This document defines the strict interface and guarantees of the Evidra Evidence Engine.

The Evidence Engine is responsible for recording immutable, append-only records of every execution attempt.

---

## 1. Evidence Model

- Evidence is append-only.
- Existing records must never be modified or deleted.
- Each record is hash-linked to the previous record.
- Integrity must be verifiable at any time.

---

## 2. Evidence Record Structure

Each execution attempt (allowed or denied) must generate one EvidenceRecord.

Minimum required fields:

{
  "event_id": "string",
  "timestamp": "RFC3339",
  "actor": {...},
  "tool": "string",
  "operation": "string",
  "params": {},
  "policy_decision": {
    "allow": true | false,
    "reason": "string"
  },
  "execution_result": {
    "status": "success | failed | denied",
    "exit_code": "integer | null"
  },
  "previous_hash": "string",
  "hash": "string"
}

---

## 3. Hash Chain Rules

- The hash must be computed over the canonical JSON representation of the record.
- The "hash" field itself must be excluded from hash calculation.
- "previous_hash" must reference the hash of the prior record.
- The first record must use a deterministic genesis value (e.g., empty string).

---

## 4. Enforcement Rules

- Evidence must be written before returning execution result to the caller.
- If execution is denied (by registry or policy), an EvidenceRecord must still be written.
- If writing evidence fails, execution must be treated as failed.

---

## 5. Integrity Verification

The system must provide a mechanism to:

- Validate the entire hash chain.
- Detect tampering or missing records.
- Fail validation if any record is altered.

---

## 6. Determinism Requirements

For identical inputs and identical execution results, the generated evidence record must be identical except for timestamp and hash linkage.

No nondeterministic data (random values, unordered maps, external timestamps beyond the primary timestamp field) may affect hash stability.
