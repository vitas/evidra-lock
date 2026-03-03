package evidra.policy

import data.evidra.policy.defaults as defaults

# ──────────────────────────────────────────────────────────
# Core detection + UX for the insufficient-context kill switch.
#
# deny/reason/hint entry points live in
# rules/deny_insufficient_context.rego (same package).
# ──────────────────────────────────────────────────────────

# ── Core detection ───────────────────────────────────────

# missing_required_data is the core detector for insufficient context.
missing_required_data(action) if {
	is_destructive(action.kind)
	not has_sufficient_context(action)
}

is_destructive(kind) if {
	ops := defaults.resolve_list_param("ops.destructive_operations")
	ops[_] == kind
}

# ── kubectl / oc ─────────────────────────────────────────

has_sufficient_context(action) if {
	action.kind == "kubectl.delete"
	defaults.action_namespace(action) != ""
}

has_sufficient_context(action) if {
	action.kind == "oc.delete"
	defaults.action_namespace(action) != ""
}

has_sufficient_context(action) if {
	action.kind == "kubectl.apply"
	defaults.action_namespace(action) != ""
	payload := object.get(action, "payload", {})
	not is_workload_resource(payload)
}

has_sufficient_context(action) if {
	action.kind == "oc.apply"
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

has_sufficient_context(action) if {
	action.kind == "oc.apply"
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

# ── Machine-readable reason codes ────────────────────────

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
	reasons := _dedupe(array.concat(nonfallback, fallback_codes))
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
	action.kind == "oc.delete"
	not defaults.action_namespace(action)
}

missing_namespace(action) if {
	action.kind == "oc.apply"
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

missing_workload_containers(action) if {
	action.kind == "oc.apply"
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

# ── UX: issue construction ──────────────────────────────

insufficient_context_issue(action) := issue if {
	missing_required_data(action)
	req := context_requirements(action.kind)
	issue := {
		"category": context_issue_category(action),
		"hint": req.hint,
		"skeleton": req.skeleton,
		"shape_hint": unsupported_shape_hint(action.kind),
		"reason_codes": missing_reasons(action),
	}
}

context_issue_category(action) := "unsupported_shape" if {
	payload_shape_unsupported(action)
}

context_issue_category(action) := "missing_required_data" if {
	not payload_shape_unsupported(action)
}

# ── UX: messages ─────────────────────────────────────────

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

# ── UX: shape validation ────────────────────────────────

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
	action.kind == "oc.delete"
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
	action.kind == "oc.apply"
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

# ── UX: hints ────────────────────────────────────────────

unsupported_shape_hint(kind) := hint if {
	kind == "kubectl.apply"
	hint := "Use a payload object in native manifest or flat form. Include namespace; for workload resources include pod spec containers[].image."
}

unsupported_shape_hint(kind) := hint if {
	kind == "oc.apply"
	hint := "Use a payload object in native manifest or flat form. Include namespace; for workload resources include pod spec containers[].image."
}

unsupported_shape_hint(kind) := hint if {
	kind == "kubectl.delete"
	hint := "Use a payload object with a string namespace (or target.namespace)."
}

unsupported_shape_hint(kind) := hint if {
	kind == "oc.delete"
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
	kind != "oc.apply"
	kind != "oc.delete"
	kind != "terraform.apply"
	kind != "terraform.destroy"
	not startswith(kind, "helm.")
	kind != "argocd.sync"
	hint := "Use a payload object with operation-specific fields."
}

# ── UX: context requirements ────────────────────────────

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

# ── internal helpers ─────────────────────────────────────

_dedupe(xs) := sorted if {
	set := {x | x := xs[_]}
	sorted := sort([v | v := set[_]])
}
