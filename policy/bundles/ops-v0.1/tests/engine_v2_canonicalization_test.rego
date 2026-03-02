package evidra.policy

import rego.v1

test_engine_v2_native_deployment_manifest_hits_privileged_rule_not_insufficient_context if {
	d := decision_for_payload(native_deployment_privileged_payload)
	not d.allow
	"k8s.privileged_container" in d.hits
	not "ops.insufficient_context" in d.hits
}

test_engine_v2_flat_passthrough_semantic_equality if {
	native := decision_for_payload(native_deployment_privileged_payload)
	flat := decision_for_payload(flat_privileged_payload)

	native.allow == flat.allow
	native.risk_level == flat.risk_level
	native.reason == flat.reason
	native.reasons == flat.reasons
	native.hits == flat.hits
	native.hints == flat.hints
}

decision_for_payload(payload) := d if {
	d := data.evidra.policy.decision with input as {
		"tool": "kubectl",
		"operation": "apply",
		"environment": "dev",
		"actions": [{
			"kind": "kubectl.apply",
			"target": "default",
			"risk_tags": [],
			"payload": payload,
		}],
	}
}

native_deployment_privileged_payload := {
	"apiVersion": "apps/v1",
	"kind": "Deployment",
	"metadata": {"name": "web", "namespace": "default"},
	"spec": {
		"template": {
			"spec": {
				"containers": [{
					"name": "app",
					"image": "nginx:1.25",
					"securityContext": {"privileged": true},
				}],
			},
		},
	},
}

flat_privileged_payload := {
	"namespace": "default",
	"resource": "deployment",
	"containers": [{
		"name": "app",
		"image": "nginx:1.25",
		"security_context": {"privileged": true},
	}],
	"init_containers": [],
	"volumes": [],
	"host_pid": false,
	"host_ipc": false,
	"host_network": false,
}
