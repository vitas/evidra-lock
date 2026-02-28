package evidra.policy

import data.evidra.policy.defaults as defaults

# ──────────────────────────────────────────────────────────
# Fail-closed: destructive operation with insufficient payload.
#
# If kind is in destructive_operations and no has_sufficient_context
# branch matches → deny. This covers both "known tool, missing fields"
# and "user-added tool without a clause" — same rule, same hint.
# ──────────────────────────────────────────────────────────

deny["ops.insufficient_context"] = msg if {
	action := input.actions[_]
	is_destructive(action.kind)
	not has_sufficient_context(action)
	msg := sprintf(
		"Destructive operation %s lacks required context. If this is a new tool, add a has_sufficient_context clause.",
		[action.kind],
	)
}

is_destructive(kind) if {
	ops := defaults.resolve_list_param("ops.destructive_operations")
	ops[_] == kind
}

# ── kubectl ──────────────────────────────────────────────

has_sufficient_context(action) if {
	action.kind == "kubectl.delete"
	defaults.action_namespace(action) != ""
}

has_sufficient_context(action) if {
	action.kind == "kubectl.apply"
	defaults.action_namespace(action) != ""
	payload := object.get(action, "payload", {})
	not is_workload_resource(payload)
}

has_sufficient_context(action) if {
	action.kind == "kubectl.apply"
	defaults.action_namespace(action) != ""
	payload := object.get(action, "payload", {})
	is_workload_resource(payload)
	has_real_containers(payload)
}

has_real_containers(payload) if {
	some c in defaults.all_containers(payload)
	object.get(c, "image", "") != ""
}

workload_kinds := {"pod", "deployment", "statefulset", "daemonset", "replicaset", "job", "cronjob"}

is_workload_resource(payload) if {
	raw := object.get(payload, "resource", object.get(payload, "kind", ""))
	resource := trim_space(lower(raw))
	workload_kinds[resource]
}

# ── terraform ────────────────────────────────────────────

has_sufficient_context(action) if {
	action.kind == "terraform.apply"
	payload := object.get(action, "payload", {})
	terraform_has_detail(payload)
}

has_sufficient_context(action) if {
	action.kind == "terraform.destroy"
	payload := object.get(action, "payload", {})
	is_number(object.get(payload, "destroy_count", null))
}

# Detail = at least one structurally plausible semantic signal.
# Counts alone (destroy_count, total_changes) do NOT satisfy this.

terraform_has_detail(payload) if {
	count(object.get(payload, "resource_types", [])) > 0
}

terraform_has_detail(payload) if {
	has_nonempty_object(object.get(payload, "s3_public_access_block", null))
}

terraform_has_detail(payload) if {
	has_nonempty_object(object.get(payload, "server_side_encryption", null))
}

terraform_has_detail(payload) if {
	has_plausible_sg_rules(object.get(payload, "security_group_rules", []))
}

terraform_has_detail(payload) if {
	has_plausible_statements(object.get(payload, "iam_policy_statements", []))
}

terraform_has_detail(payload) if {
	has_plausible_statements(object.get(payload, "trust_policy_statements", []))
}

has_nonempty_object(x) if {
	x != null
	is_object(x)
	count(x) > 0
}

has_plausible_sg_rules(rules) if {
	count(rules) > 0
	some r in rules
	is_object(r)
	plausible_sg_keys := {"cidr", "cidr_blocks", "from_port", "to_port", "protocol", "direction", "type"}
	some k in plausible_sg_keys
	r[k] != null
}

has_plausible_statements(stmts) if {
	count(stmts) > 0
	some s in stmts
	is_object(s)
	plausible_stmt_keys := {"Action", "NotAction", "Resource", "NotResource", "Effect", "Principal", "Condition"}
	some k in plausible_stmt_keys
	s[k] != null
}

# ── helm ─────────────────────────────────────────────────

has_sufficient_context(action) if {
	action.kind == "helm.upgrade"
	defaults.action_namespace(action) != ""
}

has_sufficient_context(action) if {
	action.kind == "helm.uninstall"
	defaults.action_namespace(action) != ""
}

# ── argocd ───────────────────────────────────────────────

has_sufficient_context(action) if {
	action.kind == "argocd.sync"
	payload := object.get(action, "payload", {})
	has_argocd_context(payload)
}

has_argocd_context(payload) if {
	payload.app_name != ""
}

has_argocd_context(payload) if {
	_ = payload.sync_policy
}

# ── argocd.project ───────────────────────────

has_sufficient_context(action) if {
	action.kind == "argocd.project"
	payload := object.get(action, "payload", {})
	count(payload) > 0
}
