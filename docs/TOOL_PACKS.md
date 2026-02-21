# Tool Packs (Level 1)

## What a Tool Pack Is

A tool pack is a local declarative extension unit:
- `pack.yaml` for tool/operation definitions
- optional `policy/` for policy guidance and tests

## How Packs Are Loaded

Evidra loads packs from `EVIDRA_PACKS_DIR`.

Defaults by profile:
- `ops`: `./packs/_core/ops`
- `dev`: `./packs/_core`

## pack.yaml v0.1

Each tool defines:
- `name`
- `kind` (`cli` only)
- `binary`
- `operations`
  - `name`
  - `args`
  - `params` schema (`string|int|bool`, required true/false)

Placeholder rules:
- `{{param}}` required placeholder
- `{{param?}}` optional placeholder

## Security Rules

- No shell execution.
- No free-form user command strings.
- Args must be explicit and schema-validated.
- Placeholders must match declared params.
- Use `policy/data*.json` allowlists as the primary customization mechanism for high-risk operations (for example, approved S3 delete prefixes).

## Policy Profile vs Pack Policy

The ops policy profile (`policy/profiles/ops-v0.1`) is the default decision engine in v0.1. It produces the final `allow`, `risk_level`, and `reason` outcome used by runtime execution.

Pack policies under `packs/.../policy` are tool-specific constraints and guardrails. They are useful for constraints like registry allowlists or S3 prefix allowlists tied to a specific tool surface.

For v0.1, the recommended approach is to keep most governance in the policy profile `data.json`, and use pack policy files for tool-specific allowlists and constraints.

## Minimal New Pack Example

```yaml
pack: "my-pack"
version: "0.1"
tools:
  - name: "mytool"
    kind: "cli"
    binary: "mytool"
    operations:
      - name: "status"
        args: ["status", "{{id}}"]
        params:
          id: {type: "string", required: true}
```

## Contribution Guidance

- Core packs: `packs/_core`
- Suggested community area: `packs/_contrib` (if adopted)
- Propose packs with minimal operation surface and clear policy guidance
