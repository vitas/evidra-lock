package evidra.policy.defaults

actions := [canonicalize_action(action) |
	action := input.actions[_]
]

canonicalize_action(action) := canonical if {
	should_canonicalize_action(action)
	payload := object.get(action, "payload", {})
	canonical := object.union(action, {"payload": canonicalize_k8s_payload(payload)})
}

canonicalize_action(action) := action if {
	not should_canonicalize_action(action)
}

should_canonicalize_action(action) if {
	is_object(action)
	kind := object.get(action, "kind", "")
	is_k8s_manifest_action(kind)
	payload := object.get(action, "payload", {})
	is_k8s_manifest_payload(payload)
}

is_k8s_manifest_action(kind) if {
	kind == "kubectl.apply"
}

is_k8s_manifest_action(kind) if {
	kind == "oc.apply"
}

is_k8s_manifest_payload(payload) if {
	is_object(payload)
	is_array(object.get(payload, "containers", null))
}

is_k8s_manifest_payload(payload) if {
	is_object(payload)
	is_array(object.get(payload, "init_containers", null))
}

is_k8s_manifest_payload(payload) if {
	is_object(payload)
	is_object(object.get(payload, "spec", null))
}

is_k8s_manifest_payload(payload) if {
	is_object(payload)
	is_string(object.get(payload, "kind", null))
}

is_k8s_manifest_payload(payload) if {
	is_object(payload)
	is_object(object.get(payload, "metadata", null))
}

is_k8s_manifest_payload(payload) if {
	is_object(payload)
	is_string(object.get(payload, "resource", null))
}

canonicalize_k8s_payload(payload) := {
	"namespace": payload_namespace(payload),
	"resource": payload_resource(payload),
	"containers": containers,
	"init_containers": init_containers,
	"volumes": volumes,
	"host_pid": host_pid,
	"host_ipc": host_ipc,
	"host_network": host_network,
} if {
	pod_spec := pod_spec_from_payload(payload)
	containers := [normalize_container(c) |
		c := array_or_empty(object.get(pod_spec, "containers", []))[_]
	]
	init_containers := [normalize_container(c) |
		c := init_containers_from_pod_spec(pod_spec)[_]
	]
	volumes := [normalize_volume(v) |
		v := array_or_empty(object.get(pod_spec, "volumes", []))[_]
	]
	host_pid := host_pid_from_pod_spec(pod_spec)
	host_ipc := host_ipc_from_pod_spec(pod_spec)
	host_network := host_network_from_pod_spec(pod_spec)
}

pod_spec_from_payload(payload) := pod_spec if {
	has_cronjob_pod_spec(payload)
	spec := as_object(object.get(payload, "spec", {}))
	job_template := as_object(object.get(spec, "jobTemplate", {}))
	job_spec := as_object(object.get(job_template, "spec", {}))
	template := as_object(object.get(job_spec, "template", {}))
	pod_spec := as_object(object.get(template, "spec", {}))
}

pod_spec_from_payload(payload) := pod_spec if {
	not has_cronjob_pod_spec(payload)
	has_workload_pod_spec(payload)
	spec := as_object(object.get(payload, "spec", {}))
	template := as_object(object.get(spec, "template", {}))
	pod_spec := as_object(object.get(template, "spec", {}))
}

pod_spec_from_payload(payload) := pod_spec if {
	not has_cronjob_pod_spec(payload)
	not has_workload_pod_spec(payload)
	has_pod_spec(payload)
	pod_spec := as_object(object.get(payload, "spec", {}))
}

pod_spec_from_payload(payload) := payload if {
	not has_cronjob_pod_spec(payload)
	not has_workload_pod_spec(payload)
	not has_pod_spec(payload)
	has_flat_pod_payload(payload)
}

pod_spec_from_payload(payload) := {} if {
	not has_cronjob_pod_spec(payload)
	not has_workload_pod_spec(payload)
	not has_pod_spec(payload)
	not has_flat_pod_payload(payload)
}

has_cronjob_pod_spec(payload) if {
	spec := as_object(object.get(payload, "spec", {}))
	job_template := as_object(object.get(spec, "jobTemplate", {}))
	job_spec := as_object(object.get(job_template, "spec", {}))
	template := as_object(object.get(job_spec, "template", {}))
	pod_spec := as_object(object.get(template, "spec", {}))
	count(pod_spec) > 0
}

has_workload_pod_spec(payload) if {
	spec := as_object(object.get(payload, "spec", {}))
	template := as_object(object.get(spec, "template", {}))
	pod_spec := as_object(object.get(template, "spec", {}))
	count(pod_spec) > 0
}

has_pod_spec(payload) if {
	spec := as_object(object.get(payload, "spec", {}))
	is_array(object.get(spec, "containers", null))
}

has_flat_pod_payload(payload) if {
	is_array(object.get(payload, "containers", null))
	spec := as_object(object.get(payload, "spec", {}))
	not is_array(object.get(spec, "containers", null))
}

payload_namespace(payload) := ns if {
	has_nonempty_string_field(payload, "namespace")
	ns := string_field(payload, "namespace")
}

payload_namespace(payload) := ns if {
	not has_nonempty_string_field(payload, "namespace")
	metadata := as_object(object.get(payload, "metadata", {}))
	has_nonempty_string_field(metadata, "namespace")
	ns := string_field(metadata, "namespace")
}

payload_namespace(payload) := "" if {
	not has_nonempty_string_field(payload, "namespace")
	metadata := as_object(object.get(payload, "metadata", {}))
	not has_nonempty_string_field(metadata, "namespace")
}

payload_resource(payload) := resource if {
	has_nonempty_string_field(payload, "resource")
	raw := string_field(payload, "resource")
	resource := lower(raw)
}

payload_resource(payload) := resource if {
	not has_nonempty_string_field(payload, "resource")
	has_nonempty_string_field(payload, "kind")
	raw := string_field(payload, "kind")
	resource := lower(raw)
}

payload_resource(payload) := "" if {
	not has_nonempty_string_field(payload, "resource")
	not has_nonempty_string_field(payload, "kind")
}

normalize_container(c) := normalized if {
	is_object(c)
	base := object.remove(c, ["securityContext"])
	security_context := normalize_security_context(c)
	normalized := object.union(base, {"security_context": security_context})
}

normalize_container(c) := c if {
	not is_object(c)
}

normalize_security_context(c) := renamed if {
	camel := as_object(object.get(c, "securityContext", {}))
	snake := as_object(object.get(c, "security_context", {}))
	merged := object.union(camel, snake)
	renamed := normalize_security_context_keys(merged)
}

normalize_security_context_keys(sc) := renamed if {
	step1 := rename_key(sc, "runAsUser", "run_as_user")
	step2 := rename_key(step1, "runAsNonRoot", "run_as_non_root")
	step3 := rename_key(step2, "runAsGroup", "run_as_group")
	step4 := rename_key(step3, "allowPrivilegeEscalation", "allow_privilege_escalation")
	renamed := rename_key(step4, "readOnlyRootFilesystem", "read_only_root_filesystem")
}

normalize_volume(v) := normalized if {
	is_object(v)
	has_key(v, "host_path")
	normalized := object.remove(v, ["hostPath"])
}

normalize_volume(v) := normalized if {
	is_object(v)
	not has_key(v, "host_path")
	has_key(v, "hostPath")
	normalized := rename_key(v, "hostPath", "host_path")
}

normalize_volume(v) := v if {
	is_object(v)
	not has_key(v, "host_path")
	not has_key(v, "hostPath")
}

normalize_volume(v) := v if {
	not is_object(v)
}

host_pid_from_pod_spec(pod_spec) := v if {
	has_bool_key(pod_spec, "host_pid")
	v := bool_field(pod_spec, "host_pid")
}

host_pid_from_pod_spec(pod_spec) := v if {
	not has_bool_key(pod_spec, "host_pid")
	has_bool_key(pod_spec, "hostPID")
	v := bool_field(pod_spec, "hostPID")
}

host_pid_from_pod_spec(pod_spec) := v if {
	not has_bool_key(pod_spec, "host_pid")
	not has_bool_key(pod_spec, "hostPID")
	has_bool_key(pod_spec, "hostPid")
	v := bool_field(pod_spec, "hostPid")
}

host_pid_from_pod_spec(pod_spec) := false if {
	not has_bool_key(pod_spec, "host_pid")
	not has_bool_key(pod_spec, "hostPID")
	not has_bool_key(pod_spec, "hostPid")
}

host_ipc_from_pod_spec(pod_spec) := v if {
	has_bool_key(pod_spec, "host_ipc")
	v := bool_field(pod_spec, "host_ipc")
}

host_ipc_from_pod_spec(pod_spec) := v if {
	not has_bool_key(pod_spec, "host_ipc")
	has_bool_key(pod_spec, "hostIPC")
	v := bool_field(pod_spec, "hostIPC")
}

host_ipc_from_pod_spec(pod_spec) := v if {
	not has_bool_key(pod_spec, "host_ipc")
	not has_bool_key(pod_spec, "hostIPC")
	has_bool_key(pod_spec, "hostIpc")
	v := bool_field(pod_spec, "hostIpc")
}

host_ipc_from_pod_spec(pod_spec) := false if {
	not has_bool_key(pod_spec, "host_ipc")
	not has_bool_key(pod_spec, "hostIPC")
	not has_bool_key(pod_spec, "hostIpc")
}

host_network_from_pod_spec(pod_spec) := v if {
	has_bool_key(pod_spec, "host_network")
	v := bool_field(pod_spec, "host_network")
}

host_network_from_pod_spec(pod_spec) := v if {
	not has_bool_key(pod_spec, "host_network")
	has_bool_key(pod_spec, "hostNetwork")
	v := bool_field(pod_spec, "hostNetwork")
}

host_network_from_pod_spec(pod_spec) := false if {
	not has_bool_key(pod_spec, "host_network")
	not has_bool_key(pod_spec, "hostNetwork")
}

init_containers_from_pod_spec(pod_spec) := init_containers if {
	has_key(pod_spec, "init_containers")
	init_containers := array_or_empty(object.get(pod_spec, "init_containers", []))
}

init_containers_from_pod_spec(pod_spec) := init_containers if {
	not has_key(pod_spec, "init_containers")
	has_key(pod_spec, "initContainers")
	init_containers := array_or_empty(object.get(pod_spec, "initContainers", []))
}

init_containers_from_pod_spec(pod_spec) := [] if {
	not has_key(pod_spec, "init_containers")
	not has_key(pod_spec, "initContainers")
}

bool_field(obj, key) := v if {
	has_key(obj, key)
	candidate := object.get(obj, key, null)
	is_boolean(candidate)
	v := candidate
}

array_or_empty(x) := arr if {
	is_array(x)
	arr := x
}

array_or_empty(x) := [] if {
	not is_array(x)
}

as_object(x) := x if {
	is_object(x)
}

as_object(x) := {} if {
	not is_object(x)
}

string_field(obj, key) := out if {
	has_nonempty_string_field(obj, key)
	out := trim_space(object.get(obj, key, ""))
}

has_nonempty_string_field(obj, key) if {
	has_key(obj, key)
	v := object.get(obj, key, "")
	is_string(v)
	trim_space(v) != ""
}

rename_key(obj, from, to) := renamed if {
	has_key(obj, from)
	value := object.get(obj, from, null)
	base := object.remove(obj, [from])
	renamed := object.union(base, {to: value})
}

rename_key(obj, from, to) := obj if {
	not has_key(obj, from)
}

has_bool_key(obj, key) if {
	has_key(obj, key)
	is_boolean(object.get(obj, key, null))
}

has_key(obj, key) if {
	keys := object.keys(obj)
	keys[_] == key
}
