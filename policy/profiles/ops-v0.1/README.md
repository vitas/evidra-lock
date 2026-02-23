# Ops Policy Profile v0.1

This is the default policy profile used by Evidra in `ops` profile.

Files:
- `policy.rego`: stable decision engine
- `data.json`: operation classes, environment mapping, defaults, and optional overrides

Customize behavior primarily in `data.json`.

Key knobs:
- `operation_classes` (`read`, `write`, `destructive`)
- `environments` (`dev`, `prod` aliases)
- `defaults` decisions per class/environment
- `overrides` for specific tool/operation exceptions
