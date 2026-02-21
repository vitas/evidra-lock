package evidra.policy

import rego.v1

default decision := {
  "allow": true,
  "risk_level": "low",
  "reason": "allowed_by_default"
}

decision := {
  "allow": false,
  "risk_level": "high",
  "reason": "intent_too_short"
} if {
  short_intent
}

else := {
  "allow": false,
  "risk_level": "critical",
  "reason": "k8s_apply_kube_system_blocked"
} if {
  kube_system_block
}

else := {
  "allow": false,
  "risk_level": "high",
  "reason": "terraform_public_exposure_blocked"
} if {
  terraform_public_block
}

else := {
  "allow": false,
  "risk_level": "high",
  "reason": "terraform_destroy_count_exceeds_limit"
} if {
  terraform_destroy_count_block
}

else := {
  "allow": false,
  "risk_level": "high",
  "reason": "prod_namespace_requires_change_approval"
} if {
  prod_namespace_requires_approval
}

else := {
  "allow": true,
  "risk_level": "high",
  "reason": "autonomous-execution"
} if {
  autonomous_execution
}

action := object.get(input.params, "action", {})
target := object.get(action, "target", {})
payload := object.get(action, "payload", {})

has_risk_tag(tag) if {
  some t in object.get(action, "risk_tags", [])
  t == tag
}

short_intent if {
  intent := trim_space(object.get(action, "intent", ""))
  count(intent) < 10
}

kube_system_block if {
  input.tool == "k8s"
  input.operation == "apply"
  namespace := object.get(target, "namespace", "")
  namespace == "kube-system"
  not has_risk_tag("breakglass")
}

terraform_public_block if {
  input.tool == "terraform"
  input.operation == "plan"
  object.get(payload, "publicly_exposed", false) == true
  not has_risk_tag("approved_public")
}

terraform_destroy_count_block if {
  input.tool == "terraform"
  input.operation == "plan"
  destroy_count := object.get(payload, "destroy_count", 0)
  destroy_count > 5
}

prod_namespace_requires_approval if {
  namespace := object.get(target, "namespace", "")
  namespace == "prod"
  not has_risk_tag("change-approved")
}

autonomous_execution if {
  input.actor.type == "agent"
  input.actor.origin == "mcp"
}
