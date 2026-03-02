package evidra.policy

import rego.v1

test_insufficient_context_core_missing_reasons_kubectl_delete_namespace if {
	action := {
		"kind": "kubectl.delete",
		"target": {},
		"payload": {},
	}
	reasons := data.evidra.policy.missing_reasons(action) with input as {"environment": "dev"}
	"missing_namespace" in reasons
}

test_insufficient_context_core_missing_reasons_kubectl_apply_workload_containers if {
	action := {
		"kind": "kubectl.apply",
		"target": {"namespace": "default"},
		"payload": {"namespace": "default", "resource": "deployment", "containers": []},
	}
	reasons := data.evidra.policy.missing_reasons(action) with input as {"environment": "dev"}
	"missing_workload_containers" in reasons
}

test_insufficient_context_core_missing_reasons_terraform_apply_detail if {
	action := {
		"kind": "terraform.apply",
		"target": {},
		"payload": {},
	}
	reasons := data.evidra.policy.missing_reasons(action) with input as {"environment": "dev"}
	"missing_terraform_detail" in reasons
}
