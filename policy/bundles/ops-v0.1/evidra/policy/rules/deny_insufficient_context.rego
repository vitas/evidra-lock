package evidra.policy

import data.evidra.policy.defaults as defaults

# ──────────────────────────────────────────────────────────
# Fail-closed: destructive operation with insufficient payload.
#
# Core detection and UX classification live in
# insufficient_context_core.rego (same package).
# ──────────────────────────────────────────────────────────

deny["ops.insufficient_context"] = msg if {
	action := defaults.actions[_]
	issue := insufficient_context_issue(action)
	msg := insufficient_context_message(action.kind, issue)
}

# Per-kind reason for decision.reasons.
insufficient_context_reason[msg] if {
	action := defaults.actions[_]
	issue := insufficient_context_issue(action)
	msg := insufficient_context_reason_message(action.kind, issue)
}

# Per-kind skeleton for decision.hints.
insufficient_context_hint[s] if {
	action := defaults.actions[_]
	issue := insufficient_context_issue(action)
	s := issue.skeleton
}

insufficient_context_hint[s] if {
	action := defaults.actions[_]
	issue := insufficient_context_issue(action)
	issue.category == "unsupported_shape"
	s := issue.shape_hint
}

# Machine-readable reason code for programmatic consumers.
insufficient_context_reason_code[code] if {
	action := defaults.actions[_]
	issue := insufficient_context_issue(action)
	code := issue.reason_codes[_]
}
