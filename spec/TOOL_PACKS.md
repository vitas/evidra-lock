# Tool Packs (Level 1)

Level 1 declarative tool packs allow local extension of Evidra without adding Go code.

## Model

- Packs are local directories loaded at startup via `EVIDRA_PACKS_DIR`.
- Pack tools are registered into the same registry used by built-in tools.
- OPA policy remains the governance layer for allow/risk/reason decisions.

## Pack Structure

```text
packs/<pack-name>/
  pack.yaml
  policy/            (optional)
    policy.rego
    data.json
    tests/           (optional)
  README.md          (optional)
```

## pack.yaml (v0.1)

- `pack`: pack name
- `version`: `0.1`
- `tools[]`: declarative tool definitions
  - `name`
  - `kind`: only `cli` in v0.1
  - `binary`
  - `operations[]`
    - `name`
    - `args[]`: constant args + placeholders
    - `params`: flat schema (`string|int|bool`, required true/false)

## Security Constraints

- No shell execution.
- No arbitrary command strings from clients.
- Placeholders only: `{{param}}` and `{{param?}}`.
- Placeholders must match declared params.
- Required params must be present and type-valid.
- String params reject newline and null-byte values.

## Loading

Set:

```bash
EVIDRA_PACKS_DIR=./packs
```

Packs are loaded and registered at startup. Duplicate tool names are rejected.

## Governance Relationship

Pack registration extends available tools. Policy still decides whether an invocation is allowed and with what risk/reason.
