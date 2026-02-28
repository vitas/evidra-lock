package evidra.policy

import data.evidra.policy.defaults as defaults

# ──────────────────────────────────────────────────────────
# Unknown tool guard: deny operations from unrecognized tools.
#
# If the kind is not in destructive_operations AND not a
# safe read operation for a known tool prefix → deny.
# Breakglass overrides this rule (for emergency use).
# ──────────────────────────────────────────────────────────

deny["ops.unknown_destructive"] = msg if {
	action := input.actions[_]
	not is_known_operation(action.kind)
	not is_safe_read_operation(action.kind)
	not defaults.has_tag(action, "breakglass")
	msg := sprintf("Unknown operation %s — cannot evaluate safety", [action.kind])
}

is_known_operation(kind) if {
	ops := defaults.resolve_list_param("ops.destructive_operations")
	ops[_] == kind
}

# Read-only operations: known tool prefix + safe suffix.
# known_tool_prefixes is DERIVED from destructive_operations — no separate list.
# When user adds "crossplane.apply" to destructive_operations, "crossplane"
# automatically becomes a known prefix and "crossplane.get" passes.

known_tool_prefixes[prefix] if {
	ops := defaults.resolve_list_param("ops.destructive_operations")
	kind := ops[_]
	parts := split(kind, ".")
	count(parts) == 2
	prefix := parts[0]
}

safe_suffixes := {"get", "list", "describe", "show", "diff", "plan", "status", "version"}

is_safe_read_operation(kind) if {
	parts := split(kind, ".")
	count(parts) == 2
	known_tool_prefixes[parts[0]]
	safe_suffixes[parts[1]]
}
