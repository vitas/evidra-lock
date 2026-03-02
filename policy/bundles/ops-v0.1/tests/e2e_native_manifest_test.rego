package evidra.policy

import rego.v1

test_native_deployment_privileged_deny_without_insufficient_context if {
	d := decision_for_native_manifest({
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {"name": "web", "namespace": "default"},
		"spec": {
			"template": {
				"spec": {
					"containers": [{"name": "app", "image": "nginx:1.25", "securityContext": {"privileged": true}}],
				},
			},
		},
	})
	not d.allow
	"k8s.privileged_container" in d.hits
	not "ops.insufficient_context" in d.hits
}

test_native_deployment_hostpid_deny_without_insufficient_context if {
	d := decision_for_native_manifest({
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {"name": "debug", "namespace": "default"},
		"spec": {
			"template": {
				"spec": {
					"hostPID": true,
					"containers": [{"name": "app", "image": "busybox:1.36"}],
				},
			},
		},
	})
	not d.allow
	"k8s.host_namespace_escape" in d.hits
	not "ops.insufficient_context" in d.hits
}

test_native_pod_privileged_deny_without_insufficient_context if {
	d := decision_for_native_manifest({
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {"name": "debug", "namespace": "default"},
		"spec": {
			"containers": [{"name": "app", "image": "alpine:3.20", "securityContext": {"privileged": true}}],
		},
	})
	not d.allow
	"k8s.privileged_container" in d.hits
	not "ops.insufficient_context" in d.hits
}

test_native_cronjob_privileged_deny_without_insufficient_context if {
	d := decision_for_native_manifest({
		"apiVersion": "batch/v1",
		"kind": "CronJob",
		"metadata": {"name": "nightly", "namespace": "default"},
		"spec": {
			"jobTemplate": {
				"spec": {
					"template": {
						"spec": {
							"containers": [{"name": "job", "image": "busybox:1.36", "securityContext": {"privileged": true}}],
							"restartPolicy": "OnFailure",
						},
					},
				},
			},
		},
	})
	not d.allow
	"k8s.privileged_container" in d.hits
	not "ops.insufficient_context" in d.hits
}

test_native_deployment_init_container_privileged_deny_without_insufficient_context if {
	d := decision_for_native_manifest({
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {"name": "web", "namespace": "default"},
		"spec": {
			"template": {
				"spec": {
					"containers": [{"name": "app", "image": "nginx:1.25", "securityContext": {"privileged": false}}],
					"initContainers": [{"name": "init", "image": "busybox:1.36", "securityContext": {"privileged": true}}],
				},
			},
		},
	})
	not d.allow
	"k8s.privileged_container" in d.hits
	not "ops.insufficient_context" in d.hits
}

decision_for_native_manifest(manifest) := d if {
	d := data.evidra.policy.decision with input as {
		"tool": "kubectl",
		"operation": "apply",
		"environment": "dev",
		"actions": [{
			"kind": "kubectl.apply",
			"target": "default",
			"risk_tags": [],
			"payload": manifest,
		}],
	}
}
