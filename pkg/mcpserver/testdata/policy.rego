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
  input.tool == "echo"
  input.operation == "run"
}

decision := {
  "allow": true,
  "risk_level": "low",
  "reason": "allowed_by_rule",
} if {
  input.tool == "git"
  input.operation == "status"
}
