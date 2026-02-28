package evidra.policy

# ──────────────────────────────────────────────────────────
# Truncation guard: deny when plan output was truncated.
#
# Applies to adapter flow only. In MCP-only flow (LLM fills
# payload), the agent won't send truncation flags — the
# insufficient_context rule handles that case.
# ──────────────────────────────────────────────────────────

deny["ops.truncated_context"] = "Plan output is truncated — manual review required" if {
	action := input.actions[_]
	is_destructive(action.kind)
	payload := object.get(action, "payload", {})
	is_truncated(payload)
}

is_truncated(payload) if {
	payload.resource_changes_truncated == true
}

is_truncated(payload) if {
	payload.delete_addresses_truncated == true
}

is_truncated(payload) if {
	payload.replace_addresses_truncated == true
}
