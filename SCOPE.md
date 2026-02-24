# Evidra v1-slim scope

## Mission

Deliver a single-purpose, deterministic DevOps validator that reads Terraform plan JSON (or similar diffs) and returns PASS/FAIL decisions with audit-ready metadata.

## In scope

- `evidra validate <file>` as the canonical entry point.
- The ops policy bundle under `policy/bundles/ops-v0.1/` with structured deny/warn rules and data-driven hints.
- Evidence logging under `~/.evidra/evidence` capturing every decision for later inspection.
- Clear documentation and example inputs so new users can get started in under one minute.

## Out of scope

- Introducing new CLI commands beyond the existing `validate`, `version`, and advanced helpers.
- Policy rewrites that change decision semantics without explicit design notes.
- Additional dependencies, agent SDKs, or runtime features not aligned with the core validator.

## Guideline

No new commands or policy areas without a short design note explaining why they belong in v1-slim.
