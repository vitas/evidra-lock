package evidra.policy

import data.evidra.policy.defaults as defaults
import data.evidra.policy.insufficient_context as core

# ──────────────────────────────────────────────────────────
# Fail-closed: destructive operation with insufficient payload.
#
# If kind is in destructive_operations and no has_sufficient_context
# branch matches → deny.
# UX detail: we distinguish two deny categories:
# - missing required data
# - unsupported payload shape (wrong types / structure)
# ──────────────────────────────────────────────────────────

deny["ops.insufficient_context"] = msg if {
	action := defaults.actions[_]
	issue := insufficient_context_issue(action)
	msg := insufficient_context_message(action.kind, issue)
}

# Per-kind reason for decision.reasons (richer than the deny message label).
insufficient_context_reason[msg] if {
	action := defaults.actions[_]
	issue := insufficient_context_issue(action)
	msg := insufficient_context_reason_message(action.kind, issue)
}

# Per-kind skeleton for decision.hints (actionable payload example).
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

insufficient_context_issue(action) := issue if {
	core.missing_required_data(action)
	req := context_requirements(action.kind)
	issue := {
		"category": context_issue_category(action),
		"hint": req.hint,
		"skeleton": req.skeleton,
		"shape_hint": unsupported_shape_hint(action.kind),
		"reason_codes": core.missing_reasons(action),
	}
}

context_issue_category(action) := "unsupported_shape" if {
	payload_shape_unsupported(action)
}

context_issue_category(action) := "missing_required_data" if {
	not payload_shape_unsupported(action)
}

insufficient_context_message(kind, issue) := msg if {
	issue.category == "missing_required_data"
	msg := sprintf(
		"Destructive operation %s lacks required context. %s",
		[kind, issue.hint],
	)
}

insufficient_context_message(kind, issue) := msg if {
	issue.category == "unsupported_shape"
	msg := sprintf(
		"Destructive operation %s has unsupported payload shape. %s",
		[kind, issue.shape_hint],
	)
}

insufficient_context_reason_message(kind, issue) := msg if {
	issue.category == "missing_required_data"
	msg := sprintf("%s: missing required data. %s", [kind, issue.hint])
}

insufficient_context_reason_message(kind, issue) := msg if {
	issue.category == "unsupported_shape"
	msg := sprintf("%s: unsupported payload shape. %s", [kind, issue.shape_hint])
}

payload_shape_unsupported(action) if {
	defaults.has_key(action, "payload")
	payload := object.get(action, "payload", null)
	not is_object(payload)
}

payload_shape_unsupported(action) if {
	action.kind == "kubectl.delete"
	payload := object.get(action, "payload", {})
	is_object(payload)
	defaults.has_key(payload, "namespace")
	not is_string(object.get(payload, "namespace", null))
}

payload_shape_unsupported(action) if {
	action.kind == "kubectl.apply"
	payload := object.get(action, "payload", {})
	is_object(payload)
	kubectl_apply_shape_unsupported(payload)
}

payload_shape_unsupported(action) if {
	action.kind == "terraform.apply"
	payload := object.get(action, "payload", {})
	is_object(payload)
	terraform_apply_shape_unsupported(payload)
}

payload_shape_unsupported(action) if {
	action.kind == "terraform.destroy"
	payload := object.get(action, "payload", {})
	is_object(payload)
	defaults.has_key(payload, "destroy_count")
	not is_number(object.get(payload, "destroy_count", null))
}

payload_shape_unsupported(action) if {
	startswith(action.kind, "helm.")
	payload := object.get(action, "payload", {})
	is_object(payload)
	defaults.has_key(payload, "namespace")
	not is_string(object.get(payload, "namespace", null))
}

payload_shape_unsupported(action) if {
	action.kind == "argocd.sync"
	payload := object.get(action, "payload", {})
	is_object(payload)
	defaults.has_key(payload, "app_name")
	not is_string(object.get(payload, "app_name", null))
}

payload_shape_unsupported(action) if {
	action.kind == "argocd.sync"
	payload := object.get(action, "payload", {})
	is_object(payload)
	defaults.has_key(payload, "sync_policy")
	not is_object(object.get(payload, "sync_policy", null))
}

kubectl_apply_shape_unsupported(payload) if {
	defaults.has_key(payload, "namespace")
	not is_string(object.get(payload, "namespace", null))
}

kubectl_apply_shape_unsupported(payload) if {
	defaults.has_key(payload, "resource")
	not is_string(object.get(payload, "resource", null))
}

kubectl_apply_shape_unsupported(payload) if {
	defaults.has_key(payload, "kind")
	not is_string(object.get(payload, "kind", null))
}

kubectl_apply_shape_unsupported(payload) if {
	defaults.has_key(payload, "containers")
	not is_array(object.get(payload, "containers", null))
}

kubectl_apply_shape_unsupported(payload) if {
	defaults.has_key(payload, "init_containers")
	not is_array(object.get(payload, "init_containers", null))
}

terraform_apply_shape_unsupported(payload) if {
	defaults.has_key(payload, "resource_types")
	not is_array(object.get(payload, "resource_types", null))
}

terraform_apply_shape_unsupported(payload) if {
	defaults.has_key(payload, "security_group_rules")
	not is_array(object.get(payload, "security_group_rules", null))
}

terraform_apply_shape_unsupported(payload) if {
	defaults.has_key(payload, "iam_policy_statements")
	not is_array(object.get(payload, "iam_policy_statements", null))
}

terraform_apply_shape_unsupported(payload) if {
	defaults.has_key(payload, "trust_policy_statements")
	not is_array(object.get(payload, "trust_policy_statements", null))
}

terraform_apply_shape_unsupported(payload) if {
	defaults.has_key(payload, "s3_public_access_block")
	v := object.get(payload, "s3_public_access_block", null)
	v != null
	not is_object(v)
}

terraform_apply_shape_unsupported(payload) if {
	defaults.has_key(payload, "server_side_encryption")
	v := object.get(payload, "server_side_encryption", null)
	v != null
	not is_object(v)
}

unsupported_shape_hint(kind) := hint if {
	kind == "kubectl.apply"
	hint := "Use a payload object in native manifest or flat form. Include namespace; for workload resources include pod spec containers[].image."
}

unsupported_shape_hint(kind) := hint if {
	kind == "kubectl.delete"
	hint := "Use a payload object with a string namespace (or target.namespace)."
}

unsupported_shape_hint(kind) := hint if {
	kind == "terraform.apply"
	hint := "Use typed fields: resource_types[], security_group_rules[], iam_policy_statements[], trust_policy_statements[], or s3_public_access_block{}."
}

unsupported_shape_hint(kind) := hint if {
	kind == "terraform.destroy"
	hint := "Use a payload object with numeric destroy_count."
}

unsupported_shape_hint(kind) := hint if {
	startswith(kind, "helm.")
	hint := "Use a payload object with a string namespace."
}

unsupported_shape_hint(kind) := hint if {
	kind == "argocd.sync"
	hint := "Use a payload object with app_name string or sync_policy object."
}

unsupported_shape_hint(kind) := hint if {
	kind != "kubectl.apply"
	kind != "kubectl.delete"
	kind != "terraform.apply"
	kind != "terraform.destroy"
	not startswith(kind, "helm.")
	kind != "argocd.sync"
	hint := "Use a payload object with operation-specific fields."
}

# context_requirements returns {hint, skeleton} for a kind.
# Clause 1: exact match in data.
context_requirements(kind) := req if {
	reqs := defaults.resolve_param("ops.context_requirements")
	req := reqs[kind]
}

# Clause 2: prefix wildcard match (e.g. "helm.*").
context_requirements(kind) := req if {
	reqs := defaults.resolve_param("ops.context_requirements")
	not reqs[kind]
	parts := split(kind, ".")
	wildcard := concat(".", [parts[0], "*"])
	req := reqs[wildcard]
}

# Clause 3: fallback for unknown tools.
context_requirements(kind) := req if {
	reqs := defaults.resolve_param("ops.context_requirements")
	not reqs[kind]
	parts := split(kind, ".")
	wildcard := concat(".", [parts[0], "*"])
	not reqs[wildcard]
	req := {
		"hint": "Add a has_sufficient_context clause for this tool",
		"skeleton": "{}",
	}
}
