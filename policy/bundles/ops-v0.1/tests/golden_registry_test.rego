package evidra.policy

import rego.v1

test_golden_registry_rule_ids_are_hittable if {
	ids := data.evidra.policy.golden.rule_ids
	every id in ids {
		inp := golden_probe_input(id)
		d := data.evidra.policy.decision with input as inp
		id in d.hits
	}
}

golden_probe_input("k8s.privileged_container") := {
	"tool": "kubectl",
	"operation": "apply",
	"actions": [{
		"kind": "kubectl.apply",
		"target": {"namespace": "default"},
		"risk_tags": [],
		"payload": {
			"namespace": "default",
			"resource": "deployment",
			"containers": [{
				"name": "api",
				"image": "nginx:1.27.0",
				"security_context": {"privileged": true},
			}],
		},
	}],
}

golden_probe_input("k8s.host_namespace_escape") := {
	"tool": "kubectl",
	"operation": "apply",
	"actions": [{
		"kind": "kubectl.apply",
		"target": {"namespace": "default"},
		"risk_tags": [],
		"payload": {
			"namespace": "default",
			"resource": "deployment",
			"containers": [{
				"name": "api",
				"image": "nginx:1.27.0",
			}],
			"host_pid": true,
		},
	}],
}
