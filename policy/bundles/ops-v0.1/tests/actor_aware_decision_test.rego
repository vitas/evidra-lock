package evidra.policy

import rego.v1

test_actor_aware_human_with_golden_hit_is_not_kill_switched if {
	d := data.evidra.policy.decision with input as privileged_apply_input("human", "")

	not d.allow
	d.actor_kind == "human"
	d.blocked_by_agent_kill_switch == false
	"k8s.privileged_container" in d.hits
	"k8s.privileged_container" in d.golden_hits
	d.reason == "Container has privileged security context"
}

test_actor_aware_agent_with_kill_switch_enabled_is_blocked if {
	d := data.evidra.policy.decision with input as privileged_apply_input("agent", "mcp-agent")

	not d.allow
	d.actor_kind == "agent"
	d.blocked_by_agent_kill_switch == true
	"k8s.privileged_container" in d.golden_hits
	d.reason == "Container has privileged security context"
}

test_actor_aware_agent_with_kill_switch_disabled_uses_baseline_reason if {
	d := data.evidra.policy.decision with input as privileged_apply_input("agent", "mcp-agent") with data.evidra.policy.agent_kill_switch.enabled as false

	not d.allow
	d.actor_kind == "agent"
	d.blocked_by_agent_kill_switch == false
	"k8s.privileged_container" in d.golden_hits
	d.reason == "Container has privileged security context"
}

test_actor_aware_ci_is_classified_and_gated_like_agent if {
	d := data.evidra.policy.decision with input as privileged_apply_input("ci", "mcp-agent")

	not d.allow
	d.actor_kind == "ci"
	d.blocked_by_agent_kill_switch == true
	"k8s.privileged_container" in d.golden_hits
	d.reason == "Container has privileged security context"
}

test_actor_aware_context_source_is_not_classifier if {
	d := data.evidra.policy.decision with input as privileged_apply_input("agent", "ci-pipeline")

	not d.allow
	d.actor_kind == "agent"
	d.blocked_by_agent_kill_switch == true
	"k8s.privileged_container" in d.golden_hits
}

test_actor_aware_unknown_actor_type_defaults_to_agent_for_safety if {
	d := data.evidra.policy.decision with input as privileged_apply_input("service-account", "ci-pipeline")

	not d.allow
	d.actor_kind == "agent"
	d.blocked_by_agent_kill_switch == true
	"k8s.privileged_container" in d.golden_hits
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
