package evidra.policy

import rego.v1

default decision := {
  "allow": false,
  "risk_level": "critical",
  "reason": "policy_denied_default",
}

decision := {
  "allow": true,
  "risk_level": "low",
  "reason": "allowed_by_rule",
} if {
  input.tool == "helm"
  input.operation == "version"
}

decision := {
  "allow": true,
  "risk_level": "low",
  "reason": "allowed_by_rule",
} if {
  input.tool == "helm"
  input.operation == "list"
}

decision := {
  "allow": true,
  "risk_level": "low",
  "reason": "allowed_by_rule",
} if {
  input.tool == "helm"
  input.operation == "status"
}

decision := {
  "allow": true,
  "risk_level": "high",
  "reason": "allowed_by_rule",
} if {
  input.tool == "helm"
  input.operation == "upgrade"
  object.get(input.context, "environment", "") == "dev"
}

decision := {
  "allow": true,
  "risk_level": "critical",
  "reason": "allowed_by_rule",
} if {
  input.tool == "helm"
  input.operation == "upgrade"
  object.get(input.context, "environment", "") == "prod"
}
