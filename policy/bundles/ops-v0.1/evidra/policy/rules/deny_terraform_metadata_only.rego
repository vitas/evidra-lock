# Deny terraform.apply in ops profile if payload contains only
# plan metadata (counts, resource_types) without resource-specific
# fields needed by ops domain rules (SG, IAM, S3).
#
# Self-resolving: when adapter v2 adds deep extraction or when
# LLM fills these fields in MCP mode, this rule stops firing.
package evidra.policy

import data.evidra.policy.defaults as defaults

deny["ops.terraform_metadata_only"] = msg if {
	defaults.profile_includes_ops
	action := defaults.actions[_]
	action.kind == "terraform.apply"
	has_sufficient_context(action)
	not has_deep_fields(action)
	msg := "terraform.apply payload contains only plan metadata. Ops rules for security groups, IAM, and S3 cannot evaluate."
}

has_deep_fields(action) if {
	payload := object.get(action, "payload", {})
	count(object.get(payload, "security_group_rules", [])) > 0
}

has_deep_fields(action) if {
	payload := object.get(action, "payload", {})
	count(object.get(payload, "iam_policy_statements", [])) > 0
}

has_deep_fields(action) if {
	payload := object.get(action, "payload", {})
	count(object.get(payload, "trust_policy_statements", [])) > 0
}

has_deep_fields(action) if {
	payload := object.get(action, "payload", {})
	has_nonempty_object(object.get(payload, "s3_public_access_block", null))
}

has_deep_fields(action) if {
	payload := object.get(action, "payload", {})
	has_nonempty_object(object.get(payload, "server_side_encryption", null))
}
