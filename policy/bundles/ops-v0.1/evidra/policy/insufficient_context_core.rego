package evidra.policy.insufficient_context

import data.evidra.policy.defaults as defaults

# missing_required_data is the core detector for insufficient context.
missing_required_data(action) if {
	is_destructive(action.kind)
	not has_sufficient_context(action)
}

# missing_reasons returns stable machine-readable reason codes.
missing_reasons(action) := reasons if {
	missing_required_data(action)
	namespace_codes := ["missing_namespace" | missing_namespace(action)]
	workload_codes := ["missing_workload_containers" | missing_workload_containers(action)]
	tf_detail_codes := ["missing_terraform_detail" | missing_terraform_detail(action)]
	destroy_codes := ["missing_destroy_count" | missing_destroy_count(action)]
	argocd_codes := ["missing_argocd_context" | missing_argocd_context(action)]
	project_codes := ["missing_project_payload" | missing_project_payload(action)]
	nonfallback := array.concat(
		namespace_codes,
		array.concat(
			workload_codes,
			array.concat(
				tf_detail_codes,
				array.concat(destroy_codes, array.concat(argocd_codes, project_codes)),
			),
		),
	)
	fallback_codes := ["missing_context_clause" | count(nonfallback) == 0]
	reasons := dedupe(array.concat(nonfallback, fallback_codes))
}

missing_namespace(action) if {
	action.kind == "kubectl.delete"
	not defaults.action_namespace(action)
}

missing_namespace(action) if {
	action.kind == "kubectl.apply"
	not defaults.action_namespace(action)
}

missing_namespace(action) if {
	action.kind == "helm.upgrade"
	not defaults.action_namespace(action)
}

missing_namespace(action) if {
	action.kind == "helm.uninstall"
	not defaults.action_namespace(action)
}

missing_workload_containers(action) if {
	action.kind == "kubectl.apply"
	defaults.action_namespace(action) != ""
	payload := object.get(action, "payload", {})
	is_workload_resource(payload)
	not has_real_containers(payload)
}

missing_terraform_detail(action) if {
	action.kind == "terraform.apply"
	payload := object.get(action, "payload", {})
	not terraform_has_detail(payload)
}

missing_destroy_count(action) if {
	action.kind == "terraform.destroy"
	payload := object.get(action, "payload", {})
	not is_number(object.get(payload, "destroy_count", null))
}

missing_argocd_context(action) if {
	action.kind == "argocd.sync"
	payload := object.get(action, "payload", {})
	not has_argocd_context(payload)
}

missing_project_payload(action) if {
	action.kind == "argocd.project"
	payload := object.get(action, "payload", {})
	count(payload) == 0
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

dedupe(xs) := sorted if {
	set := {x | x := xs[_]}
	sorted := sort([v | v := set[_]])
}
