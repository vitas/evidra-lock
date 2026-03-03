package evidra.policy

import rego.v1

test_actor_aware_human_with_non_overridable_hit_is_not_blocked if {
	d := data.evidra.policy.decision with input as privileged_apply_input("human", "")

	not d.allow
	d.actor_kind == "human"
	d.non_overridable_policies_enforced == false
	"k8s.privileged_container" in d.hits
	"k8s.privileged_container" in d.non_overridable_hits
	d.reason == "Container has privileged security context"
}

test_actor_aware_agent_with_non_overridable_enabled_is_blocked if {
	d := data.evidra.policy.decision with input as privileged_apply_input("agent", "mcp-agent")

	not d.allow
	d.actor_kind == "agent"
	d.non_overridable_policies_enforced == true
	"k8s.privileged_container" in d.non_overridable_hits
	d.reason == "Container has privileged security context"
}

test_actor_aware_agent_with_non_overridable_disabled_uses_baseline_reason if {
	d := data.evidra.policy.decision with input as privileged_apply_input("agent", "mcp-agent") with data.evidra.policy.non_overridable_policies.enabled as false

	not d.allow
	d.actor_kind == "agent"
	d.non_overridable_policies_enforced == false
	"k8s.privileged_container" in d.non_overridable_hits
	d.reason == "Container has privileged security context"
}

test_actor_aware_ci_is_classified_and_gated_like_agent if {
	d := data.evidra.policy.decision with input as privileged_apply_input("ci", "mcp-agent")

	not d.allow
	d.actor_kind == "ci"
	d.non_overridable_policies_enforced == true
	"k8s.privileged_container" in d.non_overridable_hits
	d.reason == "Container has privileged security context"
}

test_actor_aware_context_source_is_not_classifier if {
	d := data.evidra.policy.decision with input as privileged_apply_input("agent", "ci-pipeline")

	not d.allow
	d.actor_kind == "agent"
	d.non_overridable_policies_enforced == true
	"k8s.privileged_container" in d.non_overridable_hits
}

test_actor_aware_unknown_actor_type_is_not_inferred_as_ci_or_agent if {
	d := data.evidra.policy.decision with input as privileged_apply_input("service-account", "ci-pipeline")

	not d.allow
	d.actor_kind == "service-account"
	d.non_overridable_policies_enforced == false
	"k8s.privileged_container" in d.non_overridable_hits
}

privileged_apply_input(actor_type, source) := {
	"tool": "kubectl",
	"operation": "apply",
	"environment": "dev",
	"actor": {"type": actor_type},
	"context": {"source": source},
	"actions": [
		{
			"kind": "kubectl.apply",
			"target": "default",
			"risk_tags": [],
			"payload": {
				"namespace": "default",
				"resource": "deployment",
				"containers": [
					{
						"name": "api",
						"image": "nginx:1.27.0",
						"security_context": {"privileged": true}
					}
				]
			}
		}
	]
}
