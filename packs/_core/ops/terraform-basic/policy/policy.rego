package evidra.policy

import rego.v1

default decision := {
  "allow": false,
  "risk_level": "critical",
  "reason": "policy_denied_default",
}

decision := {
  "allow": true,
  "risk_level": "medium",
  "reason": "allowed_read_or_prepare",
} if {
  input.tool == "terraform"
  input.operation == "version"
}

decision := {
  "allow": true,
  "risk_level": "medium",
  "reason": "allowed_read_or_prepare",
} if {
  input.tool == "terraform"
  input.operation == "init"
}

decision := {
  "allow": true,
  "risk_level": "medium",
  "reason": "allowed_read_or_prepare",
} if {
  input.tool == "terraform"
  input.operation == "plan"
}

decision := {
  "allow": true,
  "risk_level": "high",
  "reason": "allowed_apply_dev",
} if {
  input.tool == "terraform"
  input.operation == "apply"
  object.get(input.context, "environment", "") == "dev"
}

decision := {
  "allow": true,
  "risk_level": "critical",
  "reason": "allowed_apply_prod",
} if {
  input.tool == "terraform"
  input.operation == "apply"
  object.get(input.context, "environment", "") == "prod"
}
