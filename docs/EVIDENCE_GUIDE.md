# Evidence Guide

## 1) Model

Evidence is append-only and tamper-evident.
Every execution attempt should produce an evidence record.

## 2) Store Layout

Default path: `~/.evidra/evidence`

Override path:
- MCP server: `--evidence-store` (or `--evidence-dir`)
- All binaries: `EVIDRA_EVIDENCE_DIR` (legacy `EVIDRA_EVIDENCE_PATH`)

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

## 4) Inspecting the log

- `cat ~/.evidra/evidence/manifest.json`
- `tail -n 1 ~/.evidra/evidence/segments/evidence-000001.jsonl`
- Use `jq` to filter: `jq '.records' ~/.evidra/evidence/manifest.json`
  (manifests contain counts, hashes, and segment metadata).

## 5) Audit Pack

Audit export should bundle evidence segments, the manifest, and policy snapshots.

## 6) Single Writer

- Evidra supports one writer process per evidence path.
- The store uses an inter-process lock file: `.evidra.lock`.
- If another process is writing, operations fail fast with `evidence_store_busy`.
- Windows note: lock enforcement is not supported in v0.1 (`evidence_lock_not_supported_on_windows`).
