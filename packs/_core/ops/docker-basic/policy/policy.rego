package evidra.policy

import rego.v1

default decision := {
  "allow": false,
  "risk_level": "critical",
  "reason": "policy_denied_default",
}

decision := {"allow": true, "risk_level": "low", "reason": "allowed_read_operation"} if {
  input.tool == "docker"
  input.operation == "images"
}

decision := {"allow": false, "risk_level": "critical", "reason": "denied_disallowed_registry"} if {
  input.tool == "docker"
  input.operation == "tag"
  valid_ref(target_ref)
  not has_allowed_registry(target_ref)
}

decision := {"allow": false, "risk_level": "critical", "reason": "denied_disallowed_tag"} if {
  input.tool == "docker"
  input.operation == "tag"
  valid_ref(target_ref)
  has_allowed_registry(target_ref)
  is_denied_tag(target_ref)
}

decision := {"allow": true, "risk_level": "medium", "reason": "allowed_tag_dev"} if {
  input.tool == "docker"
  input.operation == "tag"
  env == "dev"
  valid_ref(target_ref)
  has_allowed_registry(target_ref)
  not is_denied_tag(target_ref)
}

decision := {"allow": true, "risk_level": "high", "reason": "allowed_tag_prod"} if {
  input.tool == "docker"
  input.operation == "tag"
  env == "prod"
  valid_ref(target_ref)
  has_allowed_registry(target_ref)
  not is_denied_tag(target_ref)
}

decision := {"allow": false, "risk_level": "critical", "reason": "denied_disallowed_registry"} if {
  input.tool == "docker"
  input.operation == "push"
  valid_ref(push_ref)
  not has_allowed_registry(push_ref)
}

decision := {"allow": false, "risk_level": "critical", "reason": "denied_disallowed_tag"} if {
  input.tool == "docker"
  input.operation == "push"
  valid_ref(push_ref)
  has_allowed_registry(push_ref)
  is_denied_tag(push_ref)
}

decision := {"allow": true, "risk_level": "high", "reason": "allowed_push_dev"} if {
  input.tool == "docker"
  input.operation == "push"
  env == "dev"
  valid_ref(push_ref)
  has_allowed_registry(push_ref)
  not is_denied_tag(push_ref)
}

decision := {"allow": true, "risk_level": "critical", "reason": "allowed_push_prod"} if {
  input.tool == "docker"
  input.operation == "push"
  env == "prod"
  valid_ref(push_ref)
  has_allowed_registry(push_ref)
  not is_denied_tag(push_ref)
}

decision := {"allow": false, "risk_level": "critical", "reason": "policy_denied_default"} if {
  input.tool == "docker"
  input.operation == "push"
  not valid_ref(push_ref)
}

decision := {"allow": false, "risk_level": "critical", "reason": "policy_denied_default"} if {
  input.tool == "docker"
  input.operation == "tag"
  not valid_ref(target_ref)
}

env := object.get(input.context, "environment", "")
target_ref := object.get(input.params, "target", "")
push_ref := object.get(input.params, "image", "")

valid_ref(ref) if {
  ref != ""
  not regex.match("\\s", ref)
  not contains(ref, "\u0000")
}

has_allowed_registry(ref) if {
  some prefix in data.docker.allowed_registries
  startswith(ref, prefix)
}

is_denied_tag(ref) if {
  tag := ref_tag(ref)
  some denied in data.docker.deny_tags
  tag == denied
}

ref_tag(ref) := tag if {
  parts := split(ref, "/")
  last := parts[count(parts)-1]
  contains(last, ":")
  segs := split(last, ":")
  count(segs) >= 2
  tag := segs[count(segs)-1]
} else := "latest"

