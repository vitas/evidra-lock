# Plugins (Level 2)

Evidra uses compile-time plugins to extend the tool surface while keeping core execution flow unchanged.

## Model

- Plugins are regular Go modules compiled into the binary.
- No runtime loading (`.so`, wasm, remote) in v0.1.
- No `init()` auto-registration.
- Registration is explicit in `cmd/*` wiring.

## Contract

- Implement `pkg/plugins.ToolPlugin`:
  - `Name() string`
  - `Register(r registry.Registry) error`
- `Register` must call `RegisterTool` with explicit tool definitions.

## Adding a Plugin

1. Create `plugins/<name>/plugin.go`.
2. Implement `ToolPlugin`.
3. Define operations, param validation, and deterministic executor in tool definitions.
4. Register the plugin explicitly in `cmd/evidra-mcp/main.go`.
5. Add plugin registration tests.

## Plugin Categories

- Core plugins: shipped in this repository and enabled by default in local binaries.
- Customer-specific plugins: compiled into customer-specific builds with explicit registration.

A future external plugin repository is possible, but runtime plugin loading is out of scope for Level 2.
