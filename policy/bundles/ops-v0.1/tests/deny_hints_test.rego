package evidra.policy

import rego.v1

test_deny_hints_agent_kill_switch_block_contains_stop_and_golden_ids if {
	d := data.evidra.policy.decision with input as deny_hints_privileged_apply_input("agent", "mcp-agent")

	not d.allow
	d.blocked_by_agent_kill_switch == true
	"k8s.privileged_container" in d.golden_hits

	some h1 in d.hints
	contains(h1, "Agent Kill Switch")
	some h2 in d.hints
	contains(h2, "k8s.privileged_container")
	some h3 in d.hints
	contains(h3, "Stop. Ask for human confirmation")
}

test_deny_hints_insufficient_context_missing_data_contains_next_action_guidance if {
	inp := {
		"tool": "kubectl",
		"operation": "delete",
		"actions": [{
			"kind": "kubectl.delete",
			"target": {},
			"risk_tags": [],
			"payload": {},
		}],
	}

	d := data.evidra.policy.decision with input as inp

	not d.allow
	"ops.insufficient_context" in d.hits
	some h1 in d.hints
	contains(h1, "Missing required data for evaluation")
	some h2 in d.hints
	contains(h2, "native manifest or flat payload is accepted")
}

test_deny_hints_k8s_unsupported_shape_contains_shape_guidance if {
	inp := {
		"tool": "kubectl",
		"operation": "apply",
		"actions": [{
			"kind": "kubectl.apply",
			"target": {},
			"risk_tags": [],
			"payload": "not-an-object",
		}],
	}

	d := data.evidra.policy.decision with input as inp

	not d.allow
	"ops.insufficient_context" in d.hits
	some h1 in d.hints
	contains(h1, "Unsupported Kubernetes manifest shape")
	some h2 in d.hints
	contains(h2, "template pod spec layout")
}

deny_hints_privileged_apply_input(actor_type, source) := {
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
