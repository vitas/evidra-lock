# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog.

## [Unreleased]

- Added open-source project hygiene files (`LICENSE`, `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`, `SUPPORT.md`).
- Added GitHub templates and CI workflows.
- Added GoReleaser configuration and release workflow.
- Added `evidra version` command with build metadata fields (`version`, `commit`, `date`).

## [0.1.0] - 2026-02-20

- MCP gateway for ops tools with `execute` and `get_event`.
- OPA/Rego policy enforcement with structured decisions (`allow`, `risk_level`, `reason`).
- Ops-first runtime profile and official ops packs (`kubectl`, `helm`, `argocd`, `terraform`).
- Segmented append-only evidence store with sealing and hash-chain validation.
- Evidence utilities: `verify`, `violations`, `export` (audit pack).
- Local forwarder cursor state for tracking forwarding/export position.
- Level 1 declarative Tool Packs loader for local pack-based extension.
