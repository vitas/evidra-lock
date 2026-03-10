# Evidra-Lock Benchmark — Bootstrap Prompt for Claude Code

## How to use

Run in Claude Code with the source project available:


```
claude "Bootstrap evidra-benchmark from evidra source. Target directory: {DIR_NAME}. Follow EVIDRA_MIGRATION_MAP.md strictly."
```


DIR_NAME=/Users/vitas/git/evidra-benchmark

---

## System Prompt (copy this into Claude Code or .claude/instructions.md)

```
You are bootstrapping a new Go project from an existing codebase.

## Context

The existing project `evidra` (v0.2.0-rc5) is an OPA-based infrastructure
policy engine. The new project is a flight recorder and reliability benchmark
for infrastructure automation. It reuses ~15% of the existing code and drops
the rest (OPA, Rego, policy engine, enforce/deny logic, React UI).

## Rules

1. Read EVIDRA_MIGRATION_MAP.md FIRST. It is the single source of truth for
   what to COPY, what to DROP, and what to CREATE NEW. Follow it exactly.

2. Work in phases. After each phase, run `go build ./...` to verify compilation.
   Do not proceed to the next phase until the current phase compiles.

3. When copying files:
   - Replace ALL occurrences of `samebits.com/evidra` with `samebits.com/evidra-benchmark`
     in import paths.
   - Do NOT change logic in "COPY AS-IS" files. Only change imports.
   - For "COPY + EXTEND" files: first copy, then add new fields/types.
   - For "COPY + REFACTOR" files: first copy, then remove what the map says to remove,
     then add what the map says to add.

4. When creating new files:
   - Follow the type definitions in the migration map.
   - For canon adapters: reference the existing `pkg/mcpserver/intent.go` for
     field extraction patterns, but restructure per the map (identity vs security split).
   - For risk detectors: reference the existing `policy/bundles/ops-v0.1/*.rego` files
     for detection logic, but rewrite as pure Go functions.
   - For signals: reference EVIDRA_AGENT_RELIABILITY_BENCHMARK.md §2.
   - For scoring: reference EVIDRA_AGENT_RELIABILITY_BENCHMARK.md §3.

5. NEVER include `github.com/open-policy-agent/opa` in go.mod.
   NEVER create .rego files.
   NEVER create a pkg/policy, pkg/runtime, or pkg/mode directory.

6. After each phase, run the verification checks from migration map §7.

7. Create minimal but working code. Stub functions are acceptable for later phases.
   Each phase must compile and its tests must pass.

## Design Documents (read these when the migration map references them)

- CANONICALIZATION_CONTRACT_V1.md — adapter specs, noise lists, identity extraction
- EVIDRA_AGENT_RELIABILITY_BENCHMARK.md — signals, scoring, risk analysis
- EVIDRA_CANONICALIZATION_TEST_STRATEGY.md — golden corpus testing approach
- EVIDRA_ARCHITECTURE_OVERVIEW.md — overall architecture, data flow

## Phase Execution

### Phase 1: Foundation (must compile, evidence tests pass)

```bash
mkdir -p {TARGET}/cmd/evidra {TARGET}/cmd/evidra-mcp
mkdir -p {TARGET}/internal/evidence {TARGET}/internal/canon
mkdir -p {TARGET}/internal/risk {TARGET}/internal/signal {TARGET}/internal/score
mkdir -p {TARGET}/pkg/evidence {TARGET}/pkg/invocation {TARGET}/pkg/version
mkdir -p {TARGET}/pkg/mcpserver
mkdir -p {TARGET}/tests/golden
mkdir -p {TARGET}/docs
```

1. Create go.mod:
   - Module: samebits.com/evidra-benchmark
   - Go: 1.24.6
   - Require: go-sdk, ulid, yaml.v3 (from source go.mod)
   - Do NOT add OPA

2. Copy internal/evidence/* from source (signer, payload, builder, types + tests)
   - Fix import paths
   - Verify: `go test ./internal/evidence/...`

3. Copy pkg/evidence/* from source (io, segment, resource_links, forwarder)
   - Fix import paths

4. Copy pkg/invocation/invocation.go from source
   - Fix import paths
   - Remove `rejectUnknownKeys` and `allowedParamKeys` (OPA-specific validation)
   - Keep: Actor struct, ToolInvocation struct, basic ValidateStructure

5. Copy pkg/version/version.go from source
   - Fix import paths

6. Create minimal cmd/evidra/main.go (just `func main() { fmt.Println("evidra-benchmark") }`)

7. Verify: `go build ./...` compiles, `go test ./internal/evidence/...` passes

### Phase 2: Canonicalization (golden corpus tests pass)

1. Add k8s.io/apimachinery to go.mod:
   `go get k8s.io/apimachinery@latest`

2. Add hashicorp/terraform-json to go.mod:
   `go get github.com/hashicorp/terraform-json@latest`

3. Create internal/canon/types.go with:
   - CanonicalAction struct (per migration map)
   - CanonResult struct
   - ResourceID struct (tool-specific identity)

4. Create internal/canon/noise.go with:
   - Frozen noise field lists from CANONICALIZATION_CONTRACT_V1.md §4.5
   - removeNoiseFields function

5. Create internal/canon/k8s.go:
   - Use `k8s.io/apimachinery/pkg/runtime/serializer/yaml` for decoding
   - Use `k8s.io/apimachinery/pkg/apis/meta/v1/unstructured` for field access
   - Reference source `pkg/mcpserver/intent.go` extractK8sIntent() for field paths
   - But ONLY extract identity (apiVersion, kind, namespace, name)
   - Images, SecurityPosture → do NOT include (they go to risk detectors later)
   - Implement noise removal from the frozen list
   - Implement multi-doc YAML split and sort by identity
   - Compute intent_digest and resource_shape_hash

6. Create internal/canon/terraform.go:
   - Use `github.com/hashicorp/terraform-json`
   - Parse plan JSON into tfjson.Plan
   - Extract identity: Type + Name + Actions (NOT Address)
   - Reference source `pkg/mcpserver/intent.go` extractTerraformIntent()

7. Create internal/canon/generic.go:
   - SHA256 of raw bytes as identity and shape hash
   - resource_count = 1

8. Create tests/golden/ with at least 5 K8s cases:
   - k8s_deployment.yaml (simple deployment)
   - k8s_multidoc.yaml (2+ objects)
   - k8s_privileged.yaml (privileged container)
   - k8s_rbac.yaml (ClusterRole)
   - k8s_crd.yaml (custom resource)
   Source of example manifests: source `examples/` and `tests/corpus/`

9. Create internal/canon/canon_test.go:
   - TestGolden (with -update flag and EVIDRA_UPDATE_GOLDEN env var guard)
   - TestNoiseImmunity (5 mutators)
   - TestShapeHashSensitivity (1 test)
   - Per EVIDRA_CANONICALIZATION_TEST_STRATEGY.md

10. Generate golden digests: `EVIDRA_UPDATE_GOLDEN=1 go test -run TestGolden -update ./internal/canon/...`

11. Verify: `go test ./internal/canon/...` passes

### Phase 3: Risk Analysis (detectors pass)

1. Create internal/risk/matrix.go:
   - RiskLevel function(operationClass, scopeClass string) string
   - Fixed table from EVIDRA_AGENT_RELIABILITY_BENCHMARK.md §7

2. Create internal/risk/detectors.go:
   - Port Rego rules from source policy/bundles/ops-v0.1/ to Go
   - Read source .rego files and their tests to understand detection logic
   - Each detector: func Detect{Pattern}(rawArtifact []byte) []string
   - Start with 5 most important: privileged, hostNamespace, hostPath, massDestroy, wildcardIAM

3. Create internal/risk/risk_test.go:
   - Test matrix returns correct levels
   - Test each detector with positive and negative cases
   - Reference source .rego test files for test cases

4. Verify: `go test ./internal/risk/...` passes

### Phase 4: MCP Server (prescribe/report works)

1. Copy pkg/mcpserver/server.go from source
   - Remove validate tool handler
   - Remove all OPA references
   - Add prescribe tool handler (canon → risk → evidence → response)
   - Add report tool handler (match prescription → signals → evidence)

2. Split pkg/mcpserver/intent.go:
   - Identity extraction → already in internal/canon/k8s.go
   - Keep only IntentKey as a thin wrapper calling canon.Canonicalize
   - Or remove entirely if internal/canon handles everything

3. Adapt pkg/mcpserver/deny_cache.go → retry_tracker.go:
   - Track (intent_digest, shape_hash, timestamp) tuples
   - Fire retry signal when N repeats within T minutes

4. Create cmd/evidra-mcp/main.go:
   - Wire MCP server with canon adapters and risk analysis
   - No OPA, no bundles, no policy source

5. Verify: `go build ./cmd/evidra-mcp/` compiles

### Phase 5: Signals + Scorecard (CLI works)

1. Create internal/signal/*.go:
   - One file per signal
   - Each reads evidence chain and produces signal counts
   - Per EVIDRA_AGENT_RELIABILITY_BENCHMARK.md §2

2. Create internal/score/score.go:
   - ReliabilityScore from signal rates
   - Weighted penalty formula from EVIDRA_AGENT_RELIABILITY_BENCHMARK.md §3

3. Create internal/score/compare.go:
   - Workload overlap computation
   - Scope-aware comparison

4. Create cmd/evidra/main.go with subcommands:
   - `evidra scorecard --actor X --period 30d`
   - `evidra compare --actors X,Y --tool kubectl`
   - `evidra prescribe --artifact plan.json --tool terraform`
   - `evidra report --prescription PRS-ID --exit-code 0`

5. Verify: full `go test ./...` passes

## Final Verification

After all phases:

```bash
# No OPA anywhere
grep -r "open-policy-agent" . --include="*.go" --include="go.mod" --include="go.sum"
# Must return nothing

# No Rego files
find . -name "*.rego"
# Must return nothing

# No old import paths
grep -r "samebits.com/evidra\"" . --include="*.go"
# Must return nothing (all should be samebits.com/evidra-benchmark)

# Compiles
go build ./...

# All tests pass
go test ./... -v -count=1

# Binary size (sanity check — should be much smaller without OPA)
go build -o evidra-benchmark ./cmd/evidra/
ls -lh evidra-benchmark
# Should be < 20MB (vs ~40MB+ with OPA)
```
```
```

---

## Troubleshooting

### "cannot find module providing package X"
Run `go mod tidy` after adding/removing imports. If the missing
package is from OPA → you have a leftover import. Find and remove it.

### Evidence tests fail after copy
Check that internal/evidence/types.go doesn't reference pkg/policy.
If it does, remove that reference and replace with the new types.

### Canon test fails with "unstructured: no kind"
The K8s YAML in tests/golden/ must have apiVersion and kind at top level.
Multi-doc YAML must use `---` separator.

### go.sum conflicts
Delete go.sum and run `go mod tidy` to regenerate from scratch.
```
