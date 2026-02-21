# Ops Policy Kit v0.1

This is the default policy kit used by Evidra in `ops` profile.

Files:
- `policy.rego`: stable decision engine
- `data.json`: operation classes, environment mapping, defaults, and optional overrides
- `OPS_SURFACE.md`: current ops tool/operation surface from core ops packs

Customize behavior primarily in `data.json`.

Key knobs:
- `operation_classes` (`read`, `write`, `destructive`)
- `environments` (`dev`, `prod` aliases)
- `defaults` decisions per class/environment
- `overrides` for specific tool/operation exceptions
