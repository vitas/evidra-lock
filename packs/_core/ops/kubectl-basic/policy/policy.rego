package evidra.policy

import rego.v1

default decision := {
  "allow": false,
  "risk_level": "critical",
  "reason": "policy_denied_default",
}

decision := {"allow": true, "risk_level": "low", "reason": "allowed_read_operation"} if {
  input.tool == "kubectl"
  input.operation == "get"
}

decision := {"allow": true, "risk_level": "high", "reason": "allowed_write_dev"} if {
  input.tool == "kubectl"
  input.operation == "apply"
  object.get(input.context, "environment", "") == "dev"
}

decision := {"allow": true, "risk_level": "critical", "reason": "allowed_write_prod"} if {
  input.tool == "kubectl"
  input.operation == "apply"
  object.get(input.context, "environment", "") == "prod"
}

decision := {"allow": false, "risk_level": "critical", "reason": "policy_denied_high_risk"} if {
  input.tool == "kubectl"
  input.operation == "delete"
}

