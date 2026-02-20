# Ops Profile

`EVIDRA_PROFILE` controls default runtime registration and default pack path.

## Profiles

- `ops` (default)
  - Production-focused
  - Excludes dev/demo tools by default
  - Default packs dir: `./packs/_core/ops`

- `dev`
  - Includes dev/demo tools
  - Default packs dir: `./packs/_core`

## Examples

```bash
EVIDRA_PROFILE=ops ./evidra-mcp
EVIDRA_PROFILE=dev ./evidra-mcp
```

With explicit pack path:

```bash
EVIDRA_PROFILE=ops EVIDRA_PACKS_DIR=./packs/_core/ops ./evidra-mcp
```
