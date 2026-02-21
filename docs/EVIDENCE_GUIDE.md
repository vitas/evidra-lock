# Evidence Guide

## 1) Model

Evidence is append-only and tamper-evident.
Every execution attempt should produce an evidence record.

## 2) Store Layout

Default path: `./data/evidence`

- `manifest.json`
- `segments/evidence-000001.jsonl`, `evidence-000002.jsonl`, ...

Manifest includes:
- hash-chain summary (`last_hash`, counts)
- `sealed_segments` for completed immutable segment files
- current writable segment

## 3) Hash Chain Basics

- Each record links to previous via `previous_hash`.
- Record hash excludes its own `hash` field.
- Validation checks links across segment boundaries.
- Execution stdout/stderr are stored with truncation flags.
- Output truncation limit is controlled by `EVIDRA_MAX_OUTPUT_BYTES` (default `65536`).

## 4) Commands

```bash
go run ./cmd/evidra-evidence verify --evidence ./data/evidence
go run ./cmd/evidra-evidence violations --evidence ./data/evidence --min-risk high
go run ./cmd/evidra-evidence export --evidence ./data/evidence --out ./audit-pack.tar.gz --policy ./policy/profiles/ops-v0.1/policy.rego --data ./policy/profiles/ops-v0.1/data.json
go run ./cmd/evidra-evidence cursor show --evidence ./data/evidence
go run ./cmd/evidra-evidence cursor ack --evidence ./data/evidence --segment evidence-000001.jsonl --line 0
```

## 5) Audit Pack

Audit export includes evidence files and manifest plus policy snapshot hashes.
Always run `verify` before sharing artifacts.

## 6) Single Writer

- Evidra supports one writer process per evidence path.
- The store uses an inter-process lock file: `.evidra.lock`.
- If another process is writing, operations fail fast with `evidence_store_busy`.
- Windows note: lock enforcement is not supported in v0.1 (`evidence_lock_not_supported_on_windows`).
