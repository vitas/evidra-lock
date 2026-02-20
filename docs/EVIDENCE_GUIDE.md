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

## 4) Commands

```bash
go run ./cmd/evidra-evidence verify --evidence ./data/evidence
go run ./cmd/evidra-evidence violations --evidence ./data/evidence --min-risk high
go run ./cmd/evidra-evidence export --evidence ./data/evidence --out ./audit-pack.tar.gz --policy ./policy/policy.rego --data ./policy/data.json
go run ./cmd/evidra-evidence cursor show --evidence ./data/evidence
go run ./cmd/evidra-evidence cursor ack --evidence ./data/evidence --segment evidence-000001.jsonl --line 0
```

## 5) Audit Pack

Audit export includes evidence files and manifest plus policy snapshot hashes.
Always run `verify` before sharing artifacts.
