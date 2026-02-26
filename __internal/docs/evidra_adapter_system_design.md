# Evidra — Adapter Interface System Design

**Date:** 2026-02-26
**Status:** Ready for implementation
**First implementation:** Terraform Plan adapter (`hashicorp/terraform-json`)
**Module:** `github.com/evidra/adapters` (separate Go module, NOT part of Evidra server)

---

## 1. Problem Statement

Evidra's policy engine evaluates **business parameters** — `destroy_count`, `namespace`, `resource_types`, `image_tag`. It does not understand raw tool output like `terraform show -json`, Kubernetes manifests, or ArgoCD Application specs.

Something has to translate raw tool artifacts into business parameters. Three actors handle this translation depending on context:

| Caller | Who parses | Example |
|---|---|---|
| AI agent (MCP, Claude Code) | The LLM itself | LLM reads plan JSON, extracts "5 deletes" → `{"destroy_count": 5}` |
| CI pipeline (GitHub Actions) | A **static adapter** | Binary reads `terraform show -json`, emits structured params |
| Platform (Backstage, Slack) | The platform | Backstage already knows service/namespace — it is the data source |

Adapters serve the second case: deterministic, testable extraction of business parameters from tool-specific artifacts, for use in CI pipelines and automation where no LLM is available.

---

## 2. Design Principles

**Adapters live outside Evidra core.** The server (`github.com/evidra/evidra`) never imports adapter code. Adapters are a separate Go module with its own release cycle. When Terraform changes its plan JSON schema, only the adapter updates — the Evidra server, policy engine, and API are unaffected.

**Adapters are optional.** Any caller can POST pre-assembled `ToolInvocation` JSON directly to `/v1/validate` or `/v1/skills/{id}:execute`. Adapters are a convenience, not a requirement.

**Adapters are composable.** A GitHub Action shells out to `evidra-adapter-terraform`, pipes the JSON to `curl`, done. No SDK, no library import, no Go required.

**Adapters are pure functions.** Input bytes in, structured JSON out. No network calls, no state, no side effects. This makes them trivially testable and cacheable.

**Adapters do not call the Evidra API.** They only transform data. The caller is responsible for sending the result to Evidra. This separation means adapters work offline, in air-gapped environments, and in test pipelines.

**The `evidra` CLI is a separate binary that composes adapter + API call.** The raw pipe chain (`adapter | jq | curl`) works but is fragile — quoting hell, JSON injection risk, 4 tools to orchestrate. A dedicated CLI eliminates this:

```bash
# Future: one command replaces the entire pipe chain
evidra ci terraform --plan tfplan.bin --env production
```

This binary imports the adapter as a Go library, constructs the `ToolInvocation`, POSTs to the API, and prints a human-readable summary (or JSON with `--json`). It is NOT an adapter — it is a consumer of adapters. The purity boundary is:

| Binary | Scope | Network | Testable offline |
|---|---|---|---|
| `evidra-adapter-terraform` | Transform only | No | Yes |
| `evidra` CLI | Transform + API call + summary | Yes | Needs mock server |

The adapter remains a pure function. The CLI is an ergonomic wrapper shipped later (after Phase 0 API is stable). The pipe chain remains supported for users who prefer Unix composition or need air-gapped transform-only mode.

---

## 3. Go Interface

```go
// Package adapter defines the contract for Evidra input adapters.
//
// An adapter converts tool-specific output (terraform plan JSON,
// Kubernetes manifests, etc.) into business parameters that match
// an Evidra skill's input_schema.
package adapter

import "context"

// Result is the adapter output.
type Result struct {
    // Input contains extracted business parameters.
    // Keys and types must match the target skill's input_schema.
    // Example: {"destroy_count": 5, "resource_types": ["hcloud_server"]}
    Input map[string]any `json:"input"`

    // Metadata contains provenance information for audit logging.
    // It is NOT sent to the Evidra skill — callers may include it
    // in the actor.origin or as a separate audit record.
    Metadata map[string]any `json:"metadata,omitempty"`
}

// Adapter converts raw tool output into Evidra skill input.
type Adapter interface {
    // Name returns the adapter identifier.
    // Convention: "{tool}-{artifact}", e.g. "terraform-plan", "k8s-manifest".
    Name() string

    // Convert processes raw artifact bytes and extracts business parameters.
    //
    // Parameters:
    //   - ctx: cancellation/timeout context
    //   - raw: the artifact bytes (e.g. output of `terraform show -json`)
    //   - config: adapter-specific settings passed by the caller
    //
    // Returns:
    //   - *Result: extracted parameters + metadata
    //   - error: parse failure, unsupported format version, etc.
    Convert(ctx context.Context, raw []byte, config map[string]string) (*Result, error)
}
```

### Config Keys Convention

Config keys are adapter-specific. They use flat `snake_case` naming. Common patterns:

| Key | Default | Meaning | Scope |
|---|---|---|---|
| `filter_resource_types` | (none) | Comma-separated resource type filter | **Full** — affects counts, types, changes |
| `filter_actions` | (none) | Only include specific actions in `resource_changes` | **Detail only** — does NOT affect counts or type arrays |
| `include_data_sources` | `false` | Include data source changes | **Full** — affects counts, types, changes |
| `max_resource_changes` | `200` | Cap `resource_changes` array size | Detail only |
| `resource_changes_sort` | `address` | Sort order: `address` or `none` | Detail only |
| `truncate_strategy` | `drop_tail` | How to cap: `drop_tail` or `summary_only` | Detail only |
| `target_namespace` | (none) | Override namespace extraction | k8s-manifest only |

Unknown config keys are silently ignored — this ensures forward compatibility when an older adapter binary receives config from a newer CI action.

---

## 4. Result Schema

The `Result.Input` map is intentionally untyped (`map[string]any`) because different skills have different schemas. However, every adapter documents its output schema for the corresponding skill to validate.

### Terraform Plan Adapter Output

```json
{
  "input": {
    "create_count": 2,
    "update_count": 1,
    "destroy_count": 0,
    "replace_count": 0,
    "total_changes": 3,
    "resource_types": ["hcloud_firewall", "hcloud_server"],
    "has_destroys": false,
    "has_replaces": false,
    "drift_count": 0,
    "deferred_count": 0,
    "is_destroy_plan": false,
    "providers": ["registry.terraform.io/hetznercloud/hcloud"],

    "delete_types": [],
    "replace_types": [],
    "delete_addresses": [],
    "delete_addresses_total": 0,
    "delete_addresses_truncated": false,
    "replace_addresses": [],
    "replace_addresses_total": 0,
    "replace_addresses_truncated": false,

    "resource_changes": [
      {
        "address": "hcloud_firewall.evidra",
        "type": "hcloud_firewall",
        "action": "create",
        "provider": "registry.terraform.io/hetznercloud/hcloud"
      },
      {
        "address": "hcloud_server.evidra",
        "type": "hcloud_server",
        "action": "create",
        "provider": "registry.terraform.io/hetznercloud/hcloud"
      }
    ],
    "resource_changes_count": 3,
    "resource_changes_truncated": false
  },
  "metadata": {
    "adapter_name": "terraform-plan",
    "adapter_version": "0.1.0",
    "output_schema_version": "terraform-plan@v1",
    "terraform_version": "1.10.3",
    "format_version": "1.2",
    "timestamp": "2026-02-26T14:30:00Z",
    "resource_count": 3,
    "artifact_sha256": "a1b2c3d4e5f6...64hex"
  }
}
```

### Output Fields Reference

The output has three tiers, each with different filter/truncation behavior:

**Tier 1 — Counts (scope-filtered, never truncated, never affected by `filter_actions`)**

These are the ground truth. Policy rules should prefer these over iterating arrays.

| Field | Type | Description |
|---|---|---|
| `create_count` | int | Resources to create |
| `update_count` | int | Resources to update in-place |
| `destroy_count` | int | Resources to destroy |
| `replace_count` | int | Resources to destroy+recreate |
| `total_changes` | int | Sum of all four counts above |
| `drift_count` | int | Resources that drifted outside Terraform |
| `deferred_count` | int | Resources deferred (Terraform 1.8+) |

Invariant: `total_changes == create_count + update_count + destroy_count + replace_count`. Always holds.

**Scope exception:** `drift_count` and `deferred_count` are **not** affected by `filter_resource_types` or `include_data_sources`. They reflect the entire plan. Rationale: drift in a resource type you're not filtering for is still a policy-relevant signal — you want to know the plan has drift even if your current scope is limited. If you need scoped drift, inspect `plan.ResourceDrift` entries directly via a custom adapter.

**Tier 2 — Risk shortcuts (scope-filtered, independently truncated, never affected by `filter_actions`)**

Pre-computed fields that eliminate the need to iterate `resource_changes` in policy.

| Field | Type | Description |
|---|---|---|
| `has_destroys` | bool | `destroy_count > 0` |
| `has_replaces` | bool | `replace_count > 0` |
| `is_destroy_plan` | bool | All changes are deletes, no creates/updates |
| `delete_types` | []string | Sorted unique resource types being deleted |
| `replace_types` | []string | Sorted unique resource types being replaced |
| `delete_addresses` | []string | Addresses of resources being deleted |
| `delete_addresses_total` | int | True count before truncation |
| `delete_addresses_truncated` | bool | `true` if array was capped |
| `replace_addresses` | []string | Addresses of resources being replaced |
| `replace_addresses_total` | int | True count before truncation |
| `replace_addresses_truncated` | bool | `true` if array was capped |

Note: `delete_addresses_total` always equals `destroy_count`. `replace_addresses_total` always equals `replace_count`. These totals exist for symmetry with the truncation pattern, so consumers don't need to cross-reference.

**Tier 3 — Per-resource detail (scope-filtered, action-filtered, truncated)**

The `resource_changes` array is the most detailed view and is affected by ALL filters:

| Field | Type | Description |
|---|---|---|
| `resource_changes` | []object | Per-resource `{address, type, action, provider}` |
| `resource_changes_count` | int | Array length before truncation, **after** `filter_actions` |
| `resource_changes_truncated` | bool | `true` if array was capped |

**Why `resource_changes_count`, not `resource_changes_total`?** Naming is deliberate:

| Field | Equals | Affected by filter_actions |
|---|---|---|
| `total_changes` | sum of all action counts | **No** — full scope |
| `resource_changes_count` | `len(resource_changes)` before truncation | **Yes** — detail view |

When `filter_actions` is not set, `resource_changes_count == total_changes`. When `filter_actions=create`, `resource_changes_count` may be less than `total_changes` because only creates are in the array. The naming `_count` (not `_total`) signals "this is the count of what's in this array," not "this is the total of all changes."

**Truncation behavior (controlled by config):**

| Config key | Default | Affects |
|---|---|---|
| `max_resource_changes` | `200` | `resource_changes`, `delete_addresses`, `replace_addresses` (each independently) |
| `truncate_strategy` | `drop_tail` | `resource_changes` only. `drop_tail`: keep first N. `summary_only`: emit empty array. |
| `resource_changes_sort` | `address` | `resource_changes` order. `address`: deterministic. `none`: plan order. |

Each array has its own `_total`/`_count` and `_truncated` fields — there is no shared truncation state. A plan with 1000 deletes and 0 creates will have `delete_addresses_truncated: true` but `resource_changes_truncated: false` (if `filter_actions=create`).

Policy example for truncated data:
```rego
deny[msg] {
    input.params.payload.resource_changes_truncated == true
    msg := sprintf("Plan has %d visible changes (truncated) — requires manual review",
        [input.params.payload.resource_changes_count])
}

deny[msg] {
    input.params.payload.delete_addresses_truncated == true
    msg := sprintf("Plan deletes %d resources (list truncated) — requires manual review",
        [input.params.payload.delete_addresses_total])
}
```

### Metadata Fields (All Adapters)

Every adapter must include:

| Field | Type | Description |
|---|---|---|
| `adapter_name` | string | Adapter identifier, e.g. `"terraform-plan"` |
| `adapter_version` | string | SemVer of the adapter binary/module |
| `output_schema_version` | string | Schema contract: `"{adapter_name}@v{N}"`, e.g. `"terraform-plan@v1"` |
| `timestamp` | string | ISO 8601 UTC of when conversion ran |
| `artifact_sha256` | string | SHA-256 hex digest of the raw input bytes |

**`artifact_sha256`** proves which exact artifact the adapter processed. This enables:
- Reproducible audit: hash ties a policy decision to a specific plan file
- Caching: identical artifact → skip re-evaluation
- Tamper detection: verify the plan wasn't modified between adapter and apply

**Testability note:** The `timestamp` field uses a package-level `var Now = time.Now` function. Tests override it for deterministic golden file comparisons:
```go
terraform.Now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
```

**Schema versioning contract:**

- `output_schema_version` is a **breaking change indicator**. It increments only when fields are removed or semantics change. Adding new fields is non-breaking and does not bump the version.
- Consumers (skills, policies, CI actions) should check `output_schema_version` and fail loudly on unknown major versions rather than silently misinterpreting data.
- The `@v1` suffix is intentionally simple — not SemVer. It signals "this output conforms to schema generation 1." When we rename `destroy_count` or change `resource_changes` structure, it becomes `@v2`.
- `adapter_version` tracks the binary release (`0.1.0`, `0.2.3`). `output_schema_version` tracks the output contract. They evolve independently — many adapter versions can share the same output schema.

Additional metadata fields are adapter-specific (terraform_version, format_version, etc.).

---

## 5. Terraform Plan Adapter — Detailed Design

### 5.1 Library Choice

**`github.com/hashicorp/terraform-json`** — HashiCorp's official, decoupled Go types for `terraform show -json` output. This library:

- Is maintained by HashiCorp specifically for external consumers
- Follows the `format_version` stability contract (currently "1.2")
- Handles plan deserialization including `Actions`, `ResourceChange`, `DeferredChanges`, `ResourceDrift`
- Is used by Terraform Cloud, Spacelift, and other policy tools
- Latest version: v0.27.2 (as of Feb 2026)

**NOT used:** `github.com/hashicorp/terraform/plans` — this is Terraform's internal package, not meant for external consumption, pulls in massive dependency tree, and has no stability guarantees.

### 5.2 Implementation

```go
package terraform

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "sort"
    "strconv"
    "strings"
    "time"

    tfjson "github.com/hashicorp/terraform-json"
    "github.com/evidra/adapters/adapter"
)

// Version is the adapter version, set at build time via ldflags.
var Version = "dev"

// Now is the time function used for timestamps. Override in tests.
var Now = time.Now

// OutputSchemaVersion is the output contract identifier.
// Bump only on breaking changes (field removal, semantic change).
const OutputSchemaVersion = "terraform-plan@v1"

const (
    defaultMaxResourceChanges = 200
    defaultSort               = "address"
    defaultTruncateStrategy   = "drop_tail"
)

// PlanAdapter converts `terraform show -json` output into Evidra skill input.
type PlanAdapter struct{}

var _ adapter.Adapter = (*PlanAdapter)(nil)

func (a *PlanAdapter) Name() string { return "terraform-plan" }

func (a *PlanAdapter) Convert(
    ctx context.Context, raw []byte, config map[string]string,
) (*adapter.Result, error) {
    var plan tfjson.Plan
    if err := json.Unmarshal(raw, &plan); err != nil {
        return nil, fmt.Errorf("terraform-plan: unmarshal: %w", err)
    }
    if err := plan.Validate(); err != nil {
        return nil, fmt.Errorf("terraform-plan: validate: %w", err)
    }

    // --- Parse config ---
    includeData := config["include_data_sources"] == "true"
    filterTypes := parseCSV(config["filter_resource_types"])
    filterActions := parseCSV(config["filter_actions"])
    maxChanges := parseIntOrDefault(config["max_resource_changes"], defaultMaxResourceChanges)
    sortOrder := configOrDefault(config["resource_changes_sort"], defaultSort)
    truncateStrategy := configOrDefault(config["truncate_strategy"], defaultTruncateStrategy)

    // --- Single pass with two concerns ---
    //
    // SEMANTIC CONTRACT:
    //   filter_resource_types → filters EVERYTHING (counts, types, changes).
    //     Rationale: you asked "only look at hcloud_server" — counts should reflect that scope.
    //   filter_actions → filters ONLY the resource_changes array.
    //     Rationale: counts must reflect the full picture so policy can say
    //     "there are 5 deletes" even when resource_changes only shows creates.
    //   include_data_sources → filters EVERYTHING (data sources are not infra changes).
    //
    // This distinction matters: filter_resource_types narrows the *scope* of analysis,
    // while filter_actions narrows the *detail* you want to see. Counts always
    // reflect the full scope.

    var creates, updates, deletes, replaces int
    resourceTypes := map[string]bool{}
    providers := map[string]bool{}
    deleteTypes := map[string]bool{}
    replaceTypes := map[string]bool{}
    var deleteAddresses, replaceAddresses []string
    var changes []map[string]any

    for _, rc := range plan.ResourceChanges {
        if rc.Change == nil {
            continue
        }
        // Scope filters: exclude from everything.
        if rc.Mode == tfjson.DataResourceMode && !includeData {
            continue
        }
        if len(filterTypes) > 0 && !filterTypes[rc.Type] {
            continue
        }

        resourceTypes[rc.Type] = true
        if rc.ProviderName != "" {
            providers[rc.ProviderName] = true
        }

        action := primaryAction(rc.Change.Actions)

        // --- Always count (regardless of filter_actions) ---
        switch action {
        case "create":
            creates++
        case "update":
            updates++
        case "delete":
            deletes++
            deleteTypes[rc.Type] = true
            deleteAddresses = append(deleteAddresses, rc.Address)
        case "replace":
            replaces++
            replaceTypes[rc.Type] = true
            replaceAddresses = append(replaceAddresses, rc.Address)
        }

        // --- Detail filter: only affects resource_changes array ---
        if len(filterActions) > 0 && !filterActions[action] {
            continue
        }

        changes = append(changes, map[string]any{
            "address":  rc.Address,
            "type":     rc.Type,
            "action":   action,
            "provider": rc.ProviderName,
        })
    }

    // --- Sort (deterministic output) ---
    if sortOrder == "address" {
        sort.Slice(changes, func(i, j int) bool {
            return changes[i]["address"].(string) < changes[j]["address"].(string)
        })
        sort.Strings(deleteAddresses)
        sort.Strings(replaceAddresses)
    }

    // --- Truncate ---
    // Each array tracks its own total and truncated flag independently.
    // resource_changes is subject to filter_actions, so its total may differ
    // from total_changes. Address arrays are never filtered by filter_actions
    // (they come from counts), so they have their own totals.

    rcTotal := len(changes)
    rcTruncated := false
    if maxChanges >= 0 && rcTotal > maxChanges {
        rcTruncated = true
        if truncateStrategy == "summary_only" {
            changes = nil
        } else {
            changes = changes[:maxChanges]
        }
    }

    deleteAddrTotal := len(deleteAddresses)
    deleteAddrTruncated := false
    if maxChanges >= 0 && deleteAddrTotal > maxChanges {
        deleteAddrTruncated = true
        deleteAddresses = deleteAddresses[:maxChanges]
    }

    replaceAddrTotal := len(replaceAddresses)
    replaceAddrTruncated := false
    if maxChanges >= 0 && replaceAddrTotal > maxChanges {
        replaceAddrTruncated = true
        replaceAddresses = replaceAddresses[:maxChanges]
    }

    // --- Compose result ---
    isDestroyPlan := deletes > 0 && creates == 0 && updates == 0 && replaces == 0

    return &adapter.Result{
        Input: map[string]any{
            // Counts (always accurate within resource type scope)
            "create_count":  creates,
            "update_count":  updates,
            "destroy_count": deletes,
            "replace_count": replaces,
            "total_changes": creates + updates + deletes + replaces,

            // Classification
            "resource_types":  sortedKeys(resourceTypes),
            "providers":       sortedKeys(providers),
            "has_destroys":    deletes > 0,
            "has_replaces":    replaces > 0,
            "is_destroy_plan": isDestroyPlan,

            // NOTE: drift_count and deferred_count are NOT scope-filtered.
            // They reflect the entire plan regardless of filter_resource_types
            // or include_data_sources. This is intentional — drift in an
            // unfiltered resource type is still policy-relevant signal.
            "drift_count":     len(plan.ResourceDrift),
            "deferred_count":  len(plan.DeferredChanges),

            // Risk shortcuts (not affected by filter_actions)
            "delete_types":              sortedKeys(deleteTypes),
            "replace_types":             sortedKeys(replaceTypes),
            "delete_addresses":          deleteAddresses,
            "delete_addresses_total":    deleteAddrTotal,
            "delete_addresses_truncated": deleteAddrTruncated,
            "replace_addresses":          replaceAddresses,
            "replace_addresses_total":    replaceAddrTotal,
            "replace_addresses_truncated": replaceAddrTruncated,

            // Per-resource detail (subject to filter_actions + truncation)
            "resource_changes":           changes,
            "resource_changes_count":     rcTotal,
            "resource_changes_truncated": rcTruncated,
        },
        Metadata: map[string]any{
            "adapter_name":          "terraform-plan",
            "adapter_version":       Version,
            "output_schema_version": OutputSchemaVersion,
            "terraform_version":     plan.TerraformVersion,
            "format_version":        plan.FormatVersion,
            "resource_count":        len(plan.ResourceChanges),
            "timestamp":             Now().UTC().Format(time.RFC3339),
            "artifact_sha256":       sha256Hex(raw),
        },
    }, nil
}

// primaryAction maps tfjson.Actions to a single action string.
// Replace is detected via the compound [delete, create] or [create, delete] pattern.
func primaryAction(actions tfjson.Actions) string {
    if actions.Replace() {
        return "replace"
    }
    if actions.Create() {
        return "create"
    }
    if actions.Delete() {
        return "delete"
    }
    if actions.Update() {
        return "update"
    }
    if actions.Read() {
        return "read"
    }
    // Future-proofing: if Terraform adds new action types,
    // return "unknown" rather than silently mapping to "noop".
    if actions.NoOp() {
        return "noop"
    }
    return "unknown"
}

func sha256Hex(data []byte) string {
    h := sha256.Sum256(data)
    return hex.EncodeToString(h[:])
}

func parseCSV(s string) map[string]bool {
    if s == "" {
        return nil
    }
    m := map[string]bool{}
    for _, v := range strings.Split(s, ",") {
        v = strings.TrimSpace(v)
        if v != "" {
            m[v] = true
        }
    }
    return m
}

func sortedKeys(m map[string]bool) []string {
    keys := make([]string, 0, len(m))
    for k := range m {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    return keys
}

func parseIntOrDefault(s string, def int) int {
    if s == "" {
        return def
    }
    n, err := strconv.Atoi(s)
    if err != nil || n < 0 {
        return def
    }
    return n
}

func configOrDefault(s, def string) string {
    if s == "" {
        return def
    }
    return s
}
```

### 5.3 Key Design Decisions

**`plan.Validate()` is called.** The `terraform-json` library provides a `Validate()` method that checks the format_version and required fields. This catches corrupted or truncated plan files early with a clear error instead of nil-pointer panics deeper in the code.

**Actions are mapped to a single string.** Terraform's `Actions` is a slice (e.g. `["delete", "create"]` for replace). The adapter uses the helper methods (`Replace()`, `Create()`, etc.) from `terraform-json` to reduce this to one canonical string. This simplifies policy rules — `action == "replace"` instead of checking compound arrays.

**Two kinds of filters with different semantics.** This is a critical distinction:

| Filter | Affects counts | Affects resource_changes | Affects type/address arrays | Rationale |
|---|---|---|---|---|
| `filter_resource_types` | Yes | Yes | Yes | Narrows the **scope** — "only analyze hcloud_server resources" |
| `filter_actions` | **No** | Yes | **No** | Narrows the **detail** — "only show creates in the changes list" |
| `include_data_sources` | Yes | Yes | Yes | Scope — data sources are not infra changes |

Why `filter_actions` doesn't affect counts: a policy rule that says "block if destroy_count > 5" must see ALL deletes, even if the caller asked to only show creates in `resource_changes`. Counts reflect the full picture within the resource type scope. The `resource_changes` array is the drill-down detail — filtering it is a display concern, not a policy concern.

**Counts are always accurate within the filter scope.** `destroy_count`, `total_changes`, etc. reflect every resource that passed the scope filters (`filter_resource_types`, `include_data_sources`). They are not affected by `filter_actions` or truncation. Policy rules should prefer counts and type arrays over iterating `resource_changes`.

**Deterministic output by default.** `resource_changes` is sorted by `address` unless `resource_changes_sort=none`. This means: diffs between CI runs are stable, test fixtures don't flap, and PR comments don't noise on reorder. Counts are naturally deterministic. `resource_types`, `providers`, `delete_types`, `replace_types` are always sorted.

**Two truncation strategies.** `drop_tail` keeps the first N entries (useful when you want some detail). `summary_only` drops the entire array and relies on counts/types only (useful for very large plans where even 200 entries are too much for a PR comment). Both set `resource_changes_truncated=true` and preserve `resource_changes_count`.

**Risk shortcut fields avoid iteration in policy.** `delete_types` and `replace_types` let policy rules say `"hcloud_volume" in delete_types` without looping over `resource_changes`. `delete_addresses` and `replace_addresses` provide the specific targets for audit logging and PR comments. These arrays are also subject to `max_resource_changes` truncation.

**Data sources are excluded by default.** `terraform plan` includes data source reads as "read" actions. These are not infrastructure changes and would inflate counts. Set `include_data_sources=true` to include them.

**Schema versioning via `output_schema_version`.** The `terraform-plan@v1` identifier is a contract between the adapter and consumers (skills, policies, CI actions). It changes only on breaking changes. This decouples adapter release cadence from output contract — you can ship bug fixes without forcing consumers to update their policy rules.

**`drift_count` and `deferred_count` are not scope-filtered.** These reflect the entire plan regardless of `filter_resource_types`. Rationale: drift in an unfiltered resource type is still a policy-relevant signal. If `filter_resource_types=hcloud_server` but a `hcloud_volume` drifted, the policy should still know about it. This is the single intentional scope asymmetry — documented explicitly in both code comments and the Output Fields Reference.

**Unknown actions map to `"unknown"`, not `"noop"`.** If Terraform adds new action types in a future version, the adapter returns `"unknown"` rather than silently treating them as no-ops. The skill schema includes `"unknown"` in its enum. Policy rules can catch this: `action == "unknown"` → require manual review. This prevents silent semantic drift when Terraform evolves.

**`artifact_sha256` in metadata.** SHA-256 of the raw input bytes, computed before any parsing. This ties a policy decision to a specific plan file — reproducible audit, caching (identical artifact → skip re-evaluation), and tamper detection (verify plan wasn't modified between adapter and apply). Cheap to compute, high enterprise value.

**`var Now = time.Now` for testability.** Tests override this to produce deterministic timestamps for golden file comparisons. Not a runtime concern — just test ergonomics.

### 5.4 Corresponding Skill Definition

The Terraform adapter output is consumed by a skill registered in Evidra:

```json
{
  "name": "terraform-apply",
  "description": "Evaluate a Terraform plan before apply",
  "tool": "terraform",
  "operation": "apply",
  "input_schema": {
    "type": "object",
    "properties": {
      "create_count":     { "type": "integer", "minimum": 0 },
      "update_count":     { "type": "integer", "minimum": 0 },
      "destroy_count":    { "type": "integer", "minimum": 0 },
      "replace_count":    { "type": "integer", "minimum": 0 },
      "total_changes":    { "type": "integer", "minimum": 0 },
      "resource_types":   { "type": "array", "items": { "type": "string" } },
      "has_destroys":     { "type": "boolean" },
      "has_replaces":     { "type": "boolean" },
      "drift_count":      { "type": "integer", "minimum": 0 },
      "deferred_count":   { "type": "integer", "minimum": 0 },
      "is_destroy_plan":  { "type": "boolean" },
      "providers":        { "type": "array", "items": { "type": "string" } },
      "delete_types":     { "type": "array", "items": { "type": "string" } },
      "replace_types":    { "type": "array", "items": { "type": "string" } },
      "delete_addresses": { "type": "array", "items": { "type": "string" } },
      "delete_addresses_total": { "type": "integer", "minimum": 0 },
      "delete_addresses_truncated": { "type": "boolean" },
      "replace_addresses":{ "type": "array", "items": { "type": "string" } },
      "replace_addresses_total": { "type": "integer", "minimum": 0 },
      "replace_addresses_truncated": { "type": "boolean" },
      "resource_changes": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "address":  { "type": "string" },
            "type":     { "type": "string" },
            "action":   { "type": "string", "enum": ["create", "update", "delete", "replace", "read", "noop", "unknown"] },
            "provider": { "type": "string" }
          },
          "required": ["address", "type", "action"]
        }
      },
      "resource_changes_count":     { "type": "integer", "minimum": 0 },
      "resource_changes_truncated": { "type": "boolean" }
    },
    "required": ["total_changes", "destroy_count", "has_destroys", "resource_changes_truncated"]
  },
  "risk_tags": ["infra-mutation"],
  "default_target": {}
}
```

### 5.5 Example Policy Rules

With this adapter output, OPA policy rules become straightforward:

```rego
# Block plans with >10 deletions
deny[msg] {
    input.params.payload.destroy_count > 10
    msg := sprintf("Plan deletes %d resources — requires manual approval", [input.params.payload.destroy_count])
}

# Block destroy-only plans in production
deny[msg] {
    input.params.payload.is_destroy_plan == true
    input.environment == "production"
    msg := "Destroy-only plans are blocked in production"
}

# Warn on resource drift
warn[msg] {
    input.params.payload.drift_count > 0
    msg := sprintf("Plan includes %d drifted resources — review before apply", [input.params.payload.drift_count])
}

# Block deletion of specific resource types (uses risk shortcut — no iteration)
deny[msg] {
    "hcloud_volume" == input.params.payload.delete_types[_]
    msg := sprintf("Deleting volumes is blocked — data loss risk. Targets: %v",
        [input.params.payload.delete_addresses])
}

# Block replacement of databases
deny[msg] {
    "aws_rds_instance" == input.params.payload.replace_types[_]
    msg := "Replacing RDS instances is blocked — causes downtime"
}

# Require manual review for truncated plans (large infra)
deny[msg] {
    input.params.payload.resource_changes_truncated == true
    msg := sprintf("Plan has %d changes (output truncated) — requires manual review",
        [input.params.payload.resource_changes_count])
}

# Granular: block specific address deletion (uses resource_changes array)
deny[msg] {
    some change in input.params.payload.resource_changes
    change.action == "delete"
    change.address == "hcloud_volume.data"
    msg := "Deleting the primary data volume is forbidden"
}
```

---

## 6. CLI Binary

Each adapter ships as a standalone binary with a consistent stdin/stdout contract:

```
stdin:  raw artifact bytes
stdout: ONLY JSON — Result object on success, never mixed with errors
stderr: human-readable error messages (default) or JSON error envelope (--json-errors)
exit 0: success
exit 1: parse/validation error (bad input data)
exit 2: usage error (no input, bad flags)
```

### Error Envelope

With `--json-errors`, stderr outputs a machine-readable JSON object instead of plain text:

```json
{
  "error": {
    "code": "PARSE_ERROR",
    "message": "terraform-plan: unmarshal: unexpected end of JSON input",
    "hint": "Ensure input is from `terraform show -json`, not `terraform plan`",
    "adapter": "terraform-plan",
    "adapter_version": "0.1.0"
  }
}
```

Error codes:

| Code | Exit | Meaning |
|---|---|---|
| `PARSE_ERROR` | 1 | Input is not valid JSON or not a valid plan |
| `VALIDATION_ERROR` | 1 | Plan JSON parsed but failed `Validate()` (bad format_version, missing fields) |
| `EMPTY_INPUT` | 2 | Stdin was empty |
| `USAGE_ERROR` | 2 | Bad flags or arguments |

This is useful for GitHub Actions that want to post structured error comments on PRs.

### Implementation

```go
// cmd/evidra-adapter-terraform/main.go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "strings"

    "github.com/evidra/adapters/terraform"
)

func main() {
    jsonErrors := false
    for _, arg := range os.Args[1:] {
        switch arg {
        case "--version":
            fmt.Printf("evidra-adapter-terraform %s\n", terraform.Version)
            os.Exit(0)
        case "--json-errors":
            jsonErrors = true
        case "--help", "-h":
            fmt.Fprintf(os.Stderr, "Usage: terraform show -json tfplan.bin | evidra-adapter-terraform [--json-errors]\n")
            os.Exit(0)
        default:
            exitError(jsonErrors, "USAGE_ERROR", fmt.Sprintf("unknown flag: %s", arg), "", 2)
        }
    }

    raw, err := io.ReadAll(os.Stdin)
    if err != nil {
        exitError(jsonErrors, "USAGE_ERROR", fmt.Sprintf("read stdin: %v", err), "", 2)
    }
    if len(raw) == 0 {
        exitError(jsonErrors, "EMPTY_INPUT", "empty input",
            "Pipe terraform show -json output to stdin", 2)
    }

    // Parse config from environment variables.
    config := map[string]string{}
    envKeys := []string{
        "filter_resource_types",
        "filter_actions",
        "include_data_sources",
        "max_resource_changes",
        "resource_changes_sort",
        "truncate_strategy",
    }
    for _, key := range envKeys {
        if v := os.Getenv("EVIDRA_" + strings.ToUpper(key)); v != "" {
            config[key] = v
        }
    }

    adapter := &terraform.PlanAdapter{}
    result, err := adapter.Convert(context.Background(), raw, config)
    if err != nil {
        code := "PARSE_ERROR"
        if strings.Contains(err.Error(), "validate") {
            code = "VALIDATION_ERROR"
        }
        exitError(jsonErrors, code, err.Error(),
            "Ensure input is from `terraform show -json`, not `terraform plan`", 1)
    }

    enc := json.NewEncoder(os.Stdout)
    enc.SetIndent("", "  ")
    if err := enc.Encode(result); err != nil {
        exitError(jsonErrors, "PARSE_ERROR", fmt.Sprintf("encode result: %v", err), "", 1)
    }
}

type errorEnvelope struct {
    Error errorDetail `json:"error"`
}

type errorDetail struct {
    Code           string `json:"code"`
    Message        string `json:"message"`
    Hint           string `json:"hint,omitempty"`
    Adapter        string `json:"adapter"`
    AdapterVersion string `json:"adapter_version"`
}

func exitError(jsonMode bool, code, message, hint string, exitCode int) {
    if jsonMode {
        env := errorEnvelope{Error: errorDetail{
            Code:           code,
            Message:        message,
            Hint:           hint,
            Adapter:        "terraform-plan",
            AdapterVersion: terraform.Version,
        }}
        json.NewEncoder(os.Stderr).Encode(env)
    } else {
        fmt.Fprintf(os.Stderr, "error: %s\n", message)
        if hint != "" {
            fmt.Fprintf(os.Stderr, "hint: %s\n", hint)
        }
    }
    os.Exit(exitCode)
}
```

### Usage

```bash
# Basic
terraform show -json tfplan.bin | evidra-adapter-terraform

# With structured errors for CI
terraform show -json tfplan.bin | evidra-adapter-terraform --json-errors

# With truncation control
EVIDRA_MAX_RESOURCE_CHANGES=50 \
EVIDRA_TRUNCATE_STRATEGY=summary_only \
  terraform show -json tfplan.bin | evidra-adapter-terraform

# With filters
EVIDRA_FILTER_RESOURCE_TYPES="hcloud_server,hcloud_volume" \
  terraform show -json tfplan.bin | evidra-adapter-terraform

# Pipe to Evidra (full workflow)
ADAPTER_OUTPUT=$(terraform show -json tfplan.bin | evidra-adapter-terraform)
INPUT=$(echo "$ADAPTER_OUTPUT" | jq -r '.input')

curl -s -X POST https://evidra.rest/v1/validate \
  -H "Authorization: Bearer $EVIDRA_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{
    \"tool\": \"terraform\",
    \"operation\": \"apply\",
    \"params\": {\"payload\": $INPUT},
    \"actor\": {\"type\": \"pipeline\", \"id\": \"$GITHUB_RUN_ID\"},
    \"environment\": \"production\"
  }"
```

---

## 7. Module Structure

```
github.com/evidra/adapters/
├── go.mod                              # module github.com/evidra/adapters
├── go.sum
├── README.md
├── LICENSE                             # Apache-2.0
├── adapter/
│   ├── adapter.go                      # Adapter interface, Result type
│   └── adapter_test.go                 # Interface compliance tests
├── terraform/
│   ├── plan.go                         # PlanAdapter implementation
│   ├── plan_test.go                    # Unit tests with fixture plans
│   └── testdata/
│       ├── simple_create.json          # Plan: 2 creates, 0 deletes
│       ├── mixed_changes.json          # Plan: creates + updates + deletes
│       ├── destroy_only.json           # Plan: destroy mode
│       ├── with_drift.json             # Plan: resource drift detected
│       ├── with_deferred.json          # Plan: deferred changes (TF 1.8+)
│       ├── with_modules.json           # Plan: child modules
│       ├── large_plan.json            # Plan: 500+ resources (truncation testing)
│       ├── empty_plan.json             # Plan: no changes
│       ├── hetzner_evidra.json         # Real plan: our Hetzner infra (dogfood)
│       └── invalid.json                # Malformed JSON for error testing
├── cmd/
│   └── evidra-adapter-terraform/
│       ├── main.go                     # CLI binary
│       └── main_test.go               # Integration tests (stdin/stdout)
├── .github/
│   └── workflows/
│       ├── test.yml                    # go test ./...
│       └── release.yml                 # goreleaser on tag
├── .goreleaser.yml                     # Cross-compile binaries
└── Makefile                            # build, test, lint
```

### go.mod

```
module github.com/evidra/adapters

go 1.23

require (
    github.com/hashicorp/terraform-json v0.27.2
)
```

Minimal dependencies — only `terraform-json` for the terraform adapter. Future adapters (k8s) add their own deps. The module stays lightweight.

---

## 8. Testing Strategy

### Unit Tests (plan_test.go)

Each test loads a fixture from `testdata/`, runs `Convert()`, and asserts the output:

```go
func TestPlanAdapter_SimpleCreate(t *testing.T) {
    raw := loadFixture(t, "simple_create.json")
    adapter := &PlanAdapter{}
    result, err := adapter.Convert(context.Background(), raw, nil)
    require.NoError(t, err)

    assert.Equal(t, 2, result.Input["create_count"])
    assert.Equal(t, 0, result.Input["destroy_count"])
    assert.Equal(t, 2, result.Input["total_changes"])
    assert.Equal(t, false, result.Input["has_destroys"])
    assert.Equal(t, false, result.Input["is_destroy_plan"])
    assert.Equal(t, false, result.Input["resource_changes_truncated"])
    assert.Equal(t, 2, result.Input["resource_changes_count"])

    // Schema version
    assert.Equal(t, "terraform-plan", result.Metadata["adapter_name"])
    assert.Equal(t, "terraform-plan@v1", result.Metadata["output_schema_version"])
}

func TestPlanAdapter_DestroyOnly(t *testing.T) {
    raw := loadFixture(t, "destroy_only.json")
    adapter := &PlanAdapter{}
    result, err := adapter.Convert(context.Background(), raw, nil)
    require.NoError(t, err)

    assert.Equal(t, true, result.Input["is_destroy_plan"])
    assert.Equal(t, true, result.Input["has_destroys"])
    assert.Greater(t, result.Input["destroy_count"], 0)

    // Risk shortcuts populated
    deleteTypes := result.Input["delete_types"].([]string)
    assert.NotEmpty(t, deleteTypes)
    deleteAddrs := result.Input["delete_addresses"].([]string)
    assert.NotEmpty(t, deleteAddrs)
}

func TestPlanAdapter_RiskShortcuts(t *testing.T) {
    raw := loadFixture(t, "mixed_changes.json") // has creates, deletes, replaces
    adapter := &PlanAdapter{}
    result, err := adapter.Convert(context.Background(), raw, nil)
    require.NoError(t, err)

    // delete_types contains only types being deleted
    deleteTypes := result.Input["delete_types"].([]string)
    for _, dt := range deleteTypes {
        assert.NotEmpty(t, dt)
    }
    // replace_types contains only types being replaced
    replaceTypes := result.Input["replace_types"].([]string)
    // All arrays are sorted
    assert.IsNonDecreasing(t, deleteTypes)
    assert.IsNonDecreasing(t, replaceTypes)
}

func TestPlanAdapter_Truncation_DropTail(t *testing.T) {
    raw := loadFixture(t, "large_plan.json") // 500+ resources
    adapter := &PlanAdapter{}
    config := map[string]string{"max_resource_changes": "10"}
    result, err := adapter.Convert(context.Background(), raw, config)
    require.NoError(t, err)

    changes := result.Input["resource_changes"].([]map[string]any)
    assert.Len(t, changes, 10)
    assert.Equal(t, true, result.Input["resource_changes_truncated"])
    assert.Greater(t, result.Input["resource_changes_count"], 10)
    // Counts are still accurate (computed before truncation)
    total := result.Input["create_count"].(int) + result.Input["update_count"].(int) +
        result.Input["destroy_count"].(int) + result.Input["replace_count"].(int)
    assert.Equal(t, result.Input["total_changes"], total)
}

func TestPlanAdapter_Truncation_SummaryOnly(t *testing.T) {
    raw := loadFixture(t, "large_plan.json")
    adapter := &PlanAdapter{}
    config := map[string]string{
        "max_resource_changes": "10",
        "truncate_strategy":    "summary_only",
    }
    result, err := adapter.Convert(context.Background(), raw, config)
    require.NoError(t, err)

    // resource_changes is nil/empty with summary_only
    assert.Nil(t, result.Input["resource_changes"])
    assert.Equal(t, true, result.Input["resource_changes_truncated"])
    // But counts are still accurate
    assert.Greater(t, result.Input["total_changes"], 0)
}

func TestPlanAdapter_Sorting_Deterministic(t *testing.T) {
    raw := loadFixture(t, "mixed_changes.json")
    adapter := &PlanAdapter{}

    // Run twice — output must be identical
    r1, _ := adapter.Convert(context.Background(), raw, nil)
    r2, _ := adapter.Convert(context.Background(), raw, nil)

    c1 := r1.Input["resource_changes"].([]map[string]any)
    c2 := r2.Input["resource_changes"].([]map[string]any)
    for i := range c1 {
        assert.Equal(t, c1[i]["address"], c2[i]["address"])
    }

    // Verify sorted by address
    for i := 1; i < len(c1); i++ {
        assert.LessOrEqual(t, c1[i-1]["address"].(string), c1[i]["address"].(string))
    }
}

func TestPlanAdapter_Sorting_None(t *testing.T) {
    raw := loadFixture(t, "mixed_changes.json")
    adapter := &PlanAdapter{}
    config := map[string]string{"resource_changes_sort": "none"}
    result, err := adapter.Convert(context.Background(), raw, config)
    require.NoError(t, err)
    // Just verify it doesn't crash — order matches plan order
    assert.NotNil(t, result.Input["resource_changes"])
}

func TestPlanAdapter_FilterResourceTypes(t *testing.T) {
    raw := loadFixture(t, "mixed_changes.json")
    adapter := &PlanAdapter{}
    config := map[string]string{"filter_resource_types": "hcloud_server"}
    result, err := adapter.Convert(context.Background(), raw, config)
    require.NoError(t, err)

    types := result.Input["resource_types"].([]string)
    assert.Equal(t, []string{"hcloud_server"}, types)
}

func TestPlanAdapter_FilterActions_DoesNotAffectCounts(t *testing.T) {
    raw := loadFixture(t, "mixed_changes.json") // has creates + deletes

    adapter := &PlanAdapter{}

    // Without filter: get baseline counts
    baseline, err := adapter.Convert(context.Background(), raw, nil)
    require.NoError(t, err)
    baselineDeletes := baseline.Input["destroy_count"]
    baselineTotal := baseline.Input["total_changes"]

    // With filter_actions=create: only show creates in resource_changes
    filtered, err := adapter.Convert(context.Background(), raw,
        map[string]string{"filter_actions": "create"})
    require.NoError(t, err)

    // Counts MUST be identical — filter_actions is detail-only
    assert.Equal(t, baselineDeletes, filtered.Input["destroy_count"],
        "filter_actions must not affect destroy_count")
    assert.Equal(t, baselineTotal, filtered.Input["total_changes"],
        "filter_actions must not affect total_changes")

    // But resource_changes only contains creates
    changes := filtered.Input["resource_changes"].([]map[string]any)
    for _, c := range changes {
        assert.Equal(t, "create", c["action"],
            "resource_changes should only contain filtered actions")
    }

    // delete_types still populated (not affected by filter_actions)
    assert.Equal(t, baseline.Input["delete_types"], filtered.Input["delete_types"])
}

func TestPlanAdapter_InvalidJSON(t *testing.T) {
    adapter := &PlanAdapter{}
    _, err := adapter.Convert(context.Background(), []byte("{broken"), nil)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "unmarshal")
}

func TestPlanAdapter_EmptyPlan(t *testing.T) {
    raw := loadFixture(t, "empty_plan.json")
    adapter := &PlanAdapter{}
    result, err := adapter.Convert(context.Background(), raw, nil)
    require.NoError(t, err)

    assert.Equal(t, 0, result.Input["total_changes"])
    assert.Equal(t, false, result.Input["resource_changes_truncated"])
    assert.Equal(t, 0, result.Input["resource_changes_count"])
    assert.Empty(t, result.Input["delete_types"])
    assert.Empty(t, result.Input["replace_types"])
}
```

### Fixture Generation

Fixtures are real `terraform show -json` output from test configurations:

```bash
# Generate fixture from our own Hetzner infra (dogfooding!)
cd evidra-infra/terraform
terraform init
terraform plan -out=tfplan.bin
terraform show -json tfplan.bin > ../../evidra-adapters/terraform/testdata/hetzner_evidra.json
```

### CLI Integration Tests (main_test.go)

```go
func TestCLI_StdinStdout(t *testing.T) {
    binary := buildTestBinary(t)
    fixture := loadFixture(t, "simple_create.json")

    cmd := exec.Command(binary)
    cmd.Stdin = bytes.NewReader(fixture)
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    err := cmd.Run()
    require.NoError(t, err, "stderr: %s", stderr.String())

    var result adapter.Result
    require.NoError(t, json.Unmarshal(stdout.Bytes(), &result))
    assert.Equal(t, 2, result.Input["create_count"])
    // stdout is ONLY JSON — no extra text
    assert.True(t, json.Valid(stdout.Bytes()))
}

func TestCLI_EmptyStdin(t *testing.T) {
    binary := buildTestBinary(t)
    cmd := exec.Command(binary)
    cmd.Stdin = strings.NewReader("")
    var stderr bytes.Buffer
    cmd.Stderr = &stderr
    err := cmd.Run()
    assert.Error(t, err) // exit code 2
    assert.Contains(t, stderr.String(), "empty input")
}

func TestCLI_JsonErrors(t *testing.T) {
    binary := buildTestBinary(t)
    cmd := exec.Command(binary, "--json-errors")
    cmd.Stdin = strings.NewReader("{invalid}")
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    err := cmd.Run()
    assert.Error(t, err) // exit code 1

    // stdout must be empty on error
    assert.Empty(t, stdout.String())
    // stderr is valid JSON error envelope
    var env struct {
        Error struct {
            Code    string `json:"code"`
            Message string `json:"message"`
            Hint    string `json:"hint"`
            Adapter string `json:"adapter"`
        } `json:"error"`
    }
    require.NoError(t, json.Unmarshal(stderr.Bytes(), &env))
    assert.Equal(t, "PARSE_ERROR", env.Error.Code)
    assert.Equal(t, "terraform-plan", env.Error.Adapter)
    assert.NotEmpty(t, env.Error.Hint)
}

func TestCLI_EnvConfig(t *testing.T) {
    binary := buildTestBinary(t)
    fixture := loadFixture(t, "mixed_changes.json")

    cmd := exec.Command(binary)
    cmd.Stdin = bytes.NewReader(fixture)
    cmd.Env = append(os.Environ(),
        "EVIDRA_MAX_RESOURCE_CHANGES=2",
        "EVIDRA_TRUNCATE_STRATEGY=drop_tail",
    )
    var stdout bytes.Buffer
    cmd.Stdout = &stdout

    err := cmd.Run()
    require.NoError(t, err)

    var result adapter.Result
    require.NoError(t, json.Unmarshal(stdout.Bytes(), &result))
    changes := result.Input["resource_changes"].([]any)
    assert.LessOrEqual(t, len(changes), 2)
    assert.Equal(t, true, result.Input["resource_changes_truncated"])
}
```

### CI Pipeline

```yaml
# .github/workflows/test.yml
name: Test
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      - run: go test -race -cover ./...
      - run: go vet ./...
```

---

## 9. Release & Distribution

### goreleaser

```yaml
# .goreleaser.yml
builds:
  - id: evidra-adapter-terraform
    main: ./cmd/evidra-adapter-terraform
    binary: evidra-adapter-terraform
    env:
      - CGO_ENABLED=0
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ldflags:
      - -s -w
      - -X github.com/evidra/adapters/terraform.Version={{.Version}}

archives:
  - id: terraform
    builds: [evidra-adapter-terraform]
    name_template: "evidra-adapter-terraform_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: "checksums.txt"

release:
  github:
    owner: evidra
    name: adapters
```

### Container Image

```dockerfile
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/evidra-adapter-terraform ./cmd/evidra-adapter-terraform

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /bin/evidra-adapter-terraform /evidra-adapter-terraform
ENTRYPOINT ["/evidra-adapter-terraform"]
```

Usage: `terraform show -json tfplan.bin | docker run -i ghcr.io/evidra/adapter-terraform:latest`

---

## 10. GitHub Action Integration

Once the adapter binary exists, the GitHub Action is trivial:

```yaml
# In user's workflow:
- name: Evidra Policy Check
  run: |
    terraform show -json tfplan.bin \
      | evidra-adapter-terraform \
      | jq -r '.input' \
      | xargs -I {} curl -sf -X POST https://evidra.rest/v1/validate \
          -H "Authorization: Bearer ${{ secrets.EVIDRA_API_KEY }}" \
          -H "Content-Type: application/json" \
          -d '{"tool":"terraform","operation":"apply","params":{"payload":{}},"actor":{"type":"pipeline","id":"${{ github.run_id }}"}}'
```

A proper `evidra/action@v1` GitHub Action wraps this with error handling, status checks, and PR comments — but the adapter binary does the heavy lifting.

---

## 11. Dogfooding — Evidra Infra

Our own `evidra-infra` repo uses the Terraform adapter to validate infrastructure changes:

```
evidra-infra/terraform/ → terraform plan → terraform show -json
  → evidra-adapter-terraform → POST /v1/validate → allow/deny
```

The fixture `testdata/hetzner_evidra.json` is generated from our actual Hetzner infrastructure plan. When we add a resource to `main.tf`, the fixture updates, adapter tests run, and the adapter correctly extracts the new resource.

This is the "Evidra validates Evidra" story for the landing page.

---

## 12. Future Adapters

### k8s-manifest (Planned)

```go
type ManifestAdapter struct{}

func (a *ManifestAdapter) Name() string { return "k8s-manifest" }

// Convert parses a single Kubernetes YAML/JSON manifest.
// Extracts: kind, namespace, name, container images, resource limits,
// labels, annotations.
func (a *ManifestAdapter) Convert(
    ctx context.Context, raw []byte, config map[string]string,
) (*adapter.Result, error) {
    // Uses k8s.io/apimachinery/pkg/apis/meta/v1/unstructured
    // to parse any resource without importing the full API types.
    ...
}
```

### generic-json (Planned)

Configurable JSONPath extraction — no code required:

```bash
# Config via environment variables
EVIDRA_JSONPATH_RESOURCE_COUNT="$.resource_changes | length"
EVIDRA_JSONPATH_HAS_DELETES="$.resource_changes[?(@.change.actions[0]=='delete')] | length > 0"
cat artifact.json | evidra-adapter-generic
```

For tools without a dedicated adapter, `generic-json` provides a zero-code solution by mapping JSONPath expressions to skill input fields.

---

## 13. Implementation Steps (Summary)

| Step | What | Effort |
|---|---|---|
| 1 | Create `github.com/evidra/adapters` repo with `go.mod`, `adapter/adapter.go` | 30 min |
| 2 | Implement `terraform/plan.go` with `PlanAdapter` | 2 hours |
| 3 | Generate test fixtures from `evidra-infra/terraform` (dogfood) | 30 min |
| 4 | Write `terraform/plan_test.go` with all fixture scenarios | 2 hours |
| 5 | Implement `cmd/evidra-adapter-terraform/main.go` with `--json-errors` | 1 hour |
| 6 | CLI integration tests (stdin/stdout, structured errors, env config) | 1 hour |
| 7 | `.goreleaser.yml` + Dockerfile + CI | 1 hour |
| 8 | Register `terraform-apply` skill in Evidra (when skills API is live) | 30 min |
| 9 | Wire into `evidra-infra` CI as dogfooding demo | 1 hour |

**Total: ~1.5 days.**

**Future (after API stabilizes):**

| Step | What | Effort |
|---|---|---|
| 10 | `evidra` CLI binary: `evidra ci terraform --plan tfplan.bin --env prod` | 1 day |
| 11 | `evidra/action@v1` GitHub Action wrapping the CLI | 0.5 day |

---

## 13b. Claude Code Implementation Guide

Step-by-step instructions for Claude Code to implement the adapter module. Each step has clear inputs, outputs, and acceptance criteria. Read the full design doc above before starting.

### Prerequisites

```
- Go 1.23+
- This document loaded as context
- Access to create github.com/evidra/adapters repo (or work locally)
```

### Step 1: Scaffold the module

**Create the repo structure and Go module.**

```
mkdir -p adapter terraform/testdata cmd/evidra-adapter-terraform
```

Files to create:

1. **`go.mod`**
   ```
   module github.com/evidra/adapters
   go 1.23
   require github.com/hashicorp/terraform-json v0.27.2
   ```
   Run `go mod tidy` after.

2. **`adapter/adapter.go`** — Copy the `Adapter` interface and `Result` type exactly from §3 of this document. This is the contract — it must not change without bumping all adapters.

3. **`adapter/adapter_test.go`** — Compile-time interface compliance test:
   ```go
   // Verify PlanAdapter satisfies Adapter at compile time.
   // Actual import tested in plan_test.go.
   ```

4. **`.gitignore`**:
   ```
   bin/
   dist/
   *.test
   ```

5. **`Makefile`**:
   ```makefile
   .PHONY: test build lint fmt tidy

   test:
   	go test -race -cover ./...

   build:
   	go build -o bin/evidra-adapter-terraform ./cmd/evidra-adapter-terraform

   lint:
   	go vet ./...

   fmt:
   	gofmt -w .

   tidy:
   	go mod tidy
   ```

6. **`CLAUDE.md`** for the adapters repo:
   ```markdown
   # CLAUDE.md

   Guidance for Claude Code working with the Evidra adapters module.

   ## Commands

   ```bash
   go test -race -cover ./...                    # all tests
   go test -run TestPlanAdapter ./terraform/...   # adapter unit tests
   go test ./cmd/evidra-adapter-terraform/...     # CLI integration tests
   go build -o bin/evidra-adapter-terraform ./cmd/evidra-adapter-terraform
   make test | build | lint | fmt | tidy
   ```

   ## Architecture

   This is a **separate Go module** from the Evidra server. Zero dependency on `github.com/evidra/evidra`.

   ```
   adapter/          → Interface + Result type (contract)
   terraform/        → PlanAdapter: terraform show -json → skill input
   cmd/              → CLI binaries (stdin/stdout)
   ```

   Key design doc: `evidra_adapter_system_design.md` (in evidra-infra or shared docs).

   ### Semantic rules

   - `filter_resource_types` is a **scope** filter → affects counts, types, everything
   - `filter_actions` is a **detail** filter → affects ONLY `resource_changes` array, NOT counts
   - `drift_count`/`deferred_count` are NOT scope-filtered (intentional, see design doc)
   - Counts are always accurate within scope, regardless of truncation or filter_actions
   - `primaryAction` returns `"unknown"` for unrecognized actions (not `"noop"`)
   - `var Now` and `var Version` are package-level for test overrides

   ### Test fixtures

   In `terraform/testdata/`. Generated from real `terraform show -json` output.
   Golden file tests should override `terraform.Now` for deterministic timestamps.
   ```

**Acceptance:** `go mod tidy && go build ./...` succeeds. `adapter/adapter.go` compiles.

---

### Step 2: Implement PlanAdapter

**Create `terraform/plan.go`** — copy the full implementation from §5.2 of this document. The code is production-ready; adjust only if `terraform-json` API has changed.

Key points to verify during implementation:

- `var _ adapter.Adapter = (*PlanAdapter)(nil)` — compile-time interface check
- `var Version = "dev"` and `var Now = time.Now` — package-level for test overrides
- `const OutputSchemaVersion = "terraform-plan@v1"`
- `plan.Validate()` called after unmarshal
- Filter ordering: scope filters (resource types, data sources) before counting, `filter_actions` after counting but before `changes = append(...)`
- Sort by address is default (`resource_changes_sort`)
- Each array (`resource_changes`, `delete_addresses`, `replace_addresses`) has independent `_count`/`_total` and `_truncated`
- `sha256Hex(raw)` computed on raw bytes (before unmarshal)
- `primaryAction` returns `"unknown"` as fallback, `"noop"` only for `actions.NoOp()`

**Acceptance:** `go build ./terraform` compiles. `go vet ./terraform` passes.

---

### Step 3: Create test fixtures

**Generate `terraform/testdata/*.json` files.** These are real `terraform show -json` output.

Minimum fixtures needed:

| Fixture | Content | How to generate |
|---|---|---|
| `simple_create.json` | 2 creates, 0 deletes | Small TF config: `resource "null_resource" "a" {}` × 2 |
| `mixed_changes.json` | Creates + updates + deletes + replaces | Config with `null_resource` + `random_id` + force replacements |
| `destroy_only.json` | Only deletes | `terraform plan -destroy` |
| `with_drift.json` | Has `resource_drift` entries | Manual: add `resource_drift` array to a plan JSON |
| `with_deferred.json` | Has `deferred_changes` | Manual: add `deferred_changes` array |
| `with_modules.json` | Child modules with `module_address` | Config with a module call |
| `large_plan.json` | 500+ resource changes | Script: generate JSON with repeated entries |
| `empty_plan.json` | Zero changes | `terraform plan` with no changes |
| `hetzner_evidra.json` | Real Hetzner infra plan | From `evidra-infra/terraform` if available |
| `invalid.json` | Malformed JSON | `{"broken` |
| `data_sources.json` | Plan with data source reads | Config with `data "http" "example" {}` |

**Practical shortcut:** For fixtures that need specific plan features (drift, deferred), you can hand-craft valid plan JSON. The format is documented at https://developer.hashicorp.com/terraform/internals/json-format. Minimum valid plan:

```json
{
  "format_version": "1.2",
  "terraform_version": "1.10.0",
  "planned_values": {},
  "resource_changes": [],
  "configuration": {}
}
```

Add `resource_changes`, `resource_drift`, `deferred_changes` entries as needed.

**For `large_plan.json`:** Write a Go test helper or script that generates 500 `resource_changes` entries:

```go
func generateLargePlan(n int) []byte {
    plan := tfjson.Plan{
        FormatVersion:    "1.2",
        TerraformVersion: "1.10.0",
    }
    for i := 0; i < n; i++ {
        plan.ResourceChanges = append(plan.ResourceChanges, &tfjson.ResourceChange{
            Address:      fmt.Sprintf("null_resource.item_%04d", i),
            Type:         "null_resource",
            Name:         fmt.Sprintf("item_%04d", i),
            ProviderName: "registry.terraform.io/hashicorp/null",
            Mode:         tfjson.ManagedResourceMode,
            Change:       &tfjson.Change{Actions: tfjson.Actions{tfjson.ActionCreate}},
        })
    }
    b, _ := json.Marshal(plan)
    return b
}
```

**Acceptance:** All fixture files are valid JSON. `simple_create.json`, `empty_plan.json`, and `invalid.json` exist at minimum.

---

### Step 4: Write unit tests

**Create `terraform/plan_test.go`** — copy test cases from §8 of this document and add:

Test categories:

1. **Happy path:** `SimpleCreate`, `DestroyOnly`, `EmptyPlan`, `MixedChanges`
2. **Risk shortcuts:** `RiskShortcuts` — verify `delete_types`, `replace_types`, `delete_addresses`, etc.
3. **Filter semantics:** `FilterResourceTypes`, `FilterActions_DoesNotAffectCounts` (critical — this tests the core semantic guarantee)
4. **Truncation:** `Truncation_DropTail`, `Truncation_SummaryOnly`
5. **Sorting:** `Sorting_Deterministic` (two runs identical), `Sorting_None`
6. **Edge cases:** `InvalidJSON`, `WithDrift`, `WithDeferred`, `DataSources_ExcludedByDefault`, `DataSources_Included`
7. **Metadata:** Verify `adapter_name`, `output_schema_version`, `artifact_sha256`, `timestamp`
8. **Unknown actions:** If possible, craft a fixture with a non-standard action and verify `"unknown"` returned

Helper functions needed:

```go
func loadFixture(t *testing.T, name string) []byte {
    t.Helper()
    data, err := os.ReadFile(filepath.Join("testdata", name))
    require.NoError(t, err)
    return data
}
```

Override `Now` in tests:

```go
func TestPlanAdapter_Metadata(t *testing.T) {
    fixedTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
    origNow := Now
    Now = func() time.Time { return fixedTime }
    defer func() { Now = origNow }()

    raw := loadFixture(t, "simple_create.json")
    result, _ := (&PlanAdapter{}).Convert(context.Background(), raw, nil)

    assert.Equal(t, "2026-01-01T00:00:00Z", result.Metadata["timestamp"])
    assert.Equal(t, "terraform-plan@v1", result.Metadata["output_schema_version"])
    assert.NotEmpty(t, result.Metadata["artifact_sha256"])
}
```

**Acceptance:** `go test -race -cover ./terraform/...` — all pass, >80% coverage.

---

### Step 5: Implement CLI binary

**Create `cmd/evidra-adapter-terraform/main.go`** — copy from §6 of this document.

Key behaviors to implement:

- `--version` → print version, exit 0
- `--json-errors` → stderr outputs JSON error envelope
- `--help` / `-h` → usage text on stderr, exit 0
- Unknown flags → error, exit 2
- Empty stdin → `EMPTY_INPUT`, exit 2
- Parse failure → `PARSE_ERROR`, exit 1
- Validation failure → `VALIDATION_ERROR`, exit 1
- Success → JSON Result on stdout (indented), exit 0
- Config from env vars: `EVIDRA_FILTER_RESOURCE_TYPES`, `EVIDRA_FILTER_ACTIONS`, `EVIDRA_INCLUDE_DATA_SOURCES`, `EVIDRA_MAX_RESOURCE_CHANGES`, `EVIDRA_RESOURCE_CHANGES_SORT`, `EVIDRA_TRUNCATE_STRATEGY`
- stdout is ALWAYS valid JSON or empty. Never mix text with JSON output.
- stderr is ALWAYS text (default) or JSON error envelope (`--json-errors`). Never on stdout.

**Acceptance:** `go build -o bin/evidra-adapter-terraform ./cmd/evidra-adapter-terraform` succeeds. `echo '{}' | bin/evidra-adapter-terraform` exits with code 1 (invalid plan).

---

### Step 6: CLI integration tests

**Create `cmd/evidra-adapter-terraform/main_test.go`** — copy from §8 CLI Integration Tests.

Tests to implement:

1. `TestCLI_StdinStdout` — valid fixture → valid JSON on stdout, exit 0
2. `TestCLI_EmptyStdin` — empty stdin → error on stderr, exit 2
3. `TestCLI_JsonErrors` — invalid input + `--json-errors` → JSON error envelope on stderr, empty stdout
4. `TestCLI_EnvConfig` — set `EVIDRA_MAX_RESOURCE_CHANGES=2` → truncated output
5. `TestCLI_Version` — `--version` → version string, exit 0
6. `TestCLI_UnknownFlag` — `--bogus` → error, exit 2

Build the test binary in `TestMain` or a helper:

```go
func buildTestBinary(t *testing.T) string {
    t.Helper()
    binary := filepath.Join(t.TempDir(), "evidra-adapter-terraform")
    cmd := exec.Command("go", "build", "-o", binary, ".")
    cmd.Dir = "." // assumes test runs from cmd/evidra-adapter-terraform/
    require.NoError(t, cmd.Run())
    return binary
}
```

**Acceptance:** `go test -race ./cmd/evidra-adapter-terraform/...` — all pass.

---

### Step 7: Release infrastructure

1. **`.goreleaser.yml`** — copy from §9. Cross-compile for linux/darwin × amd64/arm64. Set ldflags for Version.

2. **`Dockerfile`** — copy from §9. Multi-stage: Go build → distroless runtime.

3. **`.github/workflows/test.yml`**:
   ```yaml
   name: Test
   on: [push, pull_request]
   jobs:
     test:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v4
         - uses: actions/setup-go@v5
           with:
             go-version: '1.23'
         - run: go test -race -cover ./...
         - run: go vet ./...
   ```

4. **`.github/workflows/release.yml`**:
   ```yaml
   name: Release
   on:
     push:
       tags: ['v*']
   jobs:
     release:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v4
           with:
             fetch-depth: 0
         - uses: actions/setup-go@v5
           with:
             go-version: '1.23'
         - uses: goreleaser/goreleaser-action@v6
           with:
             args: release --clean
           env:
             GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
   ```

5. **`README.md`** — usage examples from §6, install instructions.

6. **`LICENSE`** — Apache-2.0.

**Acceptance:** `goreleaser check` passes (if installed locally). CI workflow YAML is valid.

---

### Step 8: Register skill (when API is live)

This step depends on the Evidra API being deployed with skills support.

```bash
curl -X POST https://evidra.rest/v1/skills \
  -H "Authorization: Bearer $EVIDRA_API_KEY" \
  -H "Content-Type: application/json" \
  -d @- << 'EOF'
{skill definition JSON from §5.4 of this document}
EOF
```

**Acceptance:** `GET /v1/skills/terraform-apply` returns the registered skill.

---

### Step 9: Dogfooding CI

Add to `evidra-infra/.github/workflows/infra-validate.yml`:

```yaml
name: Validate Infrastructure
on:
  pull_request:
    paths: ['terraform/**']

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: hashicorp/setup-terraform@v3

      - name: Terraform Plan
        run: |
          cd terraform
          terraform init
          terraform plan -out=tfplan.bin
          terraform show -json tfplan.bin > tfplan.json

      - name: Install Evidra Adapter
        run: |
          curl -sL https://github.com/evidra/adapters/releases/latest/download/evidra-adapter-terraform_linux_amd64.tar.gz \
            | tar xz -C /usr/local/bin

      - name: Evidra Policy Check
        run: |
          evidra-adapter-terraform --json-errors < terraform/tfplan.json \
            | jq -r '.input' \
            | curl -sf -X POST https://evidra.rest/v1/validate \
                -H "Authorization: Bearer ${{ secrets.EVIDRA_API_KEY }}" \
                -H "Content-Type: application/json" \
                -d @-
```

Also regenerate the dogfood fixture:

```bash
cd evidra-infra/terraform
terraform show -json tfplan.bin > ../../evidra-adapters/terraform/testdata/hetzner_evidra.json
```

**Acceptance:** PR to `evidra-infra` that modifies `terraform/` triggers the workflow and gets a policy evaluation.

---

### Checklist before shipping v0.1.0

```
[ ] go test -race -cover ./... — all pass, >80% coverage
[ ] go vet ./... — clean
[ ] gofmt -d . — no diff
[ ] go mod tidy — no diff
[ ] goreleaser check — passes
[ ] Binary: echo '{"format_version":"1.2","terraform_version":"1.10.0","resource_changes":[]}' | bin/evidra-adapter-terraform — exits 0, valid JSON
[ ] Binary: echo '' | bin/evidra-adapter-terraform — exits 2
[ ] Binary: echo '{broken' | bin/evidra-adapter-terraform --json-errors 2>&1 | jq .error.code — "PARSE_ERROR"
[ ] CLAUDE.md committed in repo root
[ ] README.md with usage examples
[ ] LICENSE (Apache-2.0)
[ ] Tag v0.1.0, push, goreleaser creates release with binaries
```

---

## 14. Relationship to Architecture

This document details §4b "Input Adapters" from the main architecture doc (`architecture_evidra-api-mvp.md`). The adapter interface and terraform reference implementation described here are referenced in:

- §4b: Adapter interface, design principle, distribution model
- §7 (Code Architecture): `adapters/` module in dependency diagram
- §10 (Non-Goals): "No raw artifact parsing in core"
- `evidra_deployment_hetzner.md` §13.10: Dogfooding CI workflow

The adapter module has **zero dependency on Evidra server code**. It imports only `hashicorp/terraform-json` (for the terraform adapter) and standard library. This is enforced by being a separate Go module with its own `go.mod`.
