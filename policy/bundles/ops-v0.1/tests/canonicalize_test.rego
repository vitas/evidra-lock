package evidra.policy

import rego.v1

test_canonicalize_flat_passthrough_semantic if {
	inp := {
		"actions": [{
			"kind": "kubectl.apply",
			"payload": {
				"namespace": "default",
				"resource": "deployment",
				"containers": [{
					"name": "app",
					"image": "nginx:1.25",
					"security_context": {"run_as_user": 1000},
				}],
				"init_containers": [],
				"volumes": [],
				"host_pid": false,
				"host_ipc": false,
				"host_network": false,
			},
		}],
	}
	a := data.evidra.policy.defaults.actions[0] with input as inp
	a.payload.namespace == "default"
	a.payload.resource == "deployment"
	a.payload.containers[0].name == "app"
	a.payload.containers[0].security_context.run_as_user == 1000
	a.payload.host_pid == false
	a.payload.host_ipc == false
	a.payload.host_network == false
}

test_canonicalize_deployment_manifest_extraction if {
	inp := {
		"actions": [{
			"kind": "kubectl.apply",
			"payload": {
				"apiVersion": "apps/v1",
				"kind": "Deployment",
				"metadata": {"name": "web", "namespace": "prod"},
				"spec": {
					"template": {
						"spec": {
							"hostNetwork": true,
							"containers": [{
								"name": "app",
								"image": "nginx:1.25",
								"securityContext": {
									"runAsUser": 0,
									"allowPrivilegeEscalation": false,
								},
							}],
							"initContainers": [{
								"name": "init",
								"image": "busybox:1.36",
								"securityContext": {"runAsNonRoot": true},
							}],
							"volumes": [{"name": "host-vol", "hostPath": {"path": "/"}}],
						},
					},
				},
			},
		}],
	}
	a := data.evidra.policy.defaults.actions[0] with input as inp
	a.payload.namespace == "prod"
	a.payload.resource == "deployment"
	a.payload.containers[0].security_context.run_as_user == 0
	a.payload.containers[0].security_context.allow_privilege_escalation == false
	a.payload.init_containers[0].security_context.run_as_non_root == true
	a.payload.volumes[0].host_path.path == "/"
	a.payload.host_network == true
}

test_canonicalize_pod_manifest_extraction if {
	inp := {
		"actions": [{
			"kind": "kubectl.apply",
			"payload": {
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {"name": "debug", "namespace": "ops"},
				"spec": {
					"hostPID": true,
					"hostIPC": true,
					"hostNetwork": false,
					"containers": [{"name": "app", "image": "busybox:1.36"}],
				},
			},
		}],
	}
	a := data.evidra.policy.defaults.actions[0] with input as inp
	a.payload.namespace == "ops"
	a.payload.resource == "pod"
	a.payload.host_pid == true
	a.payload.host_ipc == true
	a.payload.host_network == false
	a.payload.containers[0].name == "app"
}

test_canonicalize_cronjob_manifest_extraction if {
	inp := {
		"actions": [{
			"kind": "kubectl.apply",
			"payload": {
				"apiVersion": "batch/v1",
				"kind": "CronJob",
				"metadata": {"name": "nightly", "namespace": "batch"},
				"spec": {
					"jobTemplate": {
						"spec": {
							"template": {
								"spec": {
									"containers": [{"name": "job", "image": "busybox:1.36"}],
									"restartPolicy": "OnFailure",
								},
							},
						},
					},
				},
			},
		}],
	}
	a := data.evidra.policy.defaults.actions[0] with input as inp
	a.payload.namespace == "batch"
	a.payload.resource == "cronjob"
	a.payload.containers[0].name == "job"
}

test_canonicalize_security_context_casing_normalization if {
	inp := {
		"actions": [{
			"kind": "kubectl.apply",
			"payload": {
				"kind": "Pod",
				"metadata": {"namespace": "default"},
				"spec": {
					"containers": [{
						"name": "app",
						"image": "nginx:1.25",
						"securityContext": {
							"runAsUser": 1000,
							"runAsNonRoot": true,
							"runAsGroup": 2000,
							"allowPrivilegeEscalation": false,
							"readOnlyRootFilesystem": true,
							"capabilities": {"add": ["NET_BIND_SERVICE"]},
						},
					}],
				},
			},
		}],
	}
	a := data.evidra.policy.defaults.actions[0] with input as inp
	c := a.payload.containers[0]
	c.security_context.run_as_user == 1000
	c.security_context.run_as_non_root == true
	c.security_context.run_as_group == 2000
	c.security_context.allow_privilege_escalation == false
	c.security_context.read_only_root_filesystem == true
	c.security_context.capabilities.add[0] == "NET_BIND_SERVICE"
	not c.securityContext
}

test_native_privileged_container_denies_correct_rule_without_insufficient_context if {
	d := data.evidra.policy.decision with input as {
		"tool": "kubectl",
		"operation": "apply",
		"environment": "dev",
		"actions": [{
			"kind": "kubectl.apply",
			"target": "default",
			"risk_tags": [],
			"payload": {
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
			},
		}],
	}
	not d.allow
	"k8s.privileged_container" in d.hits
	not "ops.insufficient_context" in d.hits
}

test_native_cronjob_does_not_trigger_insufficient_context if {
	d := data.evidra.policy.decision with input as {
		"tool": "kubectl",
		"operation": "apply",
		"environment": "dev",
		"actions": [{
			"kind": "kubectl.apply",
			"target": "default",
			"risk_tags": [],
			"payload": {
				"apiVersion": "batch/v1",
				"kind": "CronJob",
				"metadata": {"name": "nightly", "namespace": "default"},
				"spec": {
					"jobTemplate": {
						"spec": {
							"template": {
								"spec": {
									"containers": [{
										"name": "job",
										"image": "busybox:1.36",
										"securityContext": {"runAsUser": 1000},
										"resources": {"limits": {"cpu": "100m", "memory": "128Mi"}},
									}],
									"restartPolicy": "OnFailure",
								},
							},
						},
					},
				},
			},
		}],
	}
	d.allow
	not "ops.insufficient_context" in d.hits
}
