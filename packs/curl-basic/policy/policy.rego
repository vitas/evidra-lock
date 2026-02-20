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
  input.tool == "curl"
  input.operation == "version"
}

decision := {
  "allow": true,
  "risk_level": "medium",
  "reason": "allowed_by_rule",
} if {
  input.tool == "curl"
  input.operation == "get"
}

decision := {
  "allow": true,
  "risk_level": "high",
  "reason": "allowed_by_rule",
} if {
  input.tool == "curl"
  input.operation == "post"
  object.get(input.context, "environment", "") == "dev"
}
