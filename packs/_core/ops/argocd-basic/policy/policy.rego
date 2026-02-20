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
  "reason": "allowed_read_operation",
} if {
  input.tool == "argocd"
  input.operation == "version"
}

decision := {
  "allow": true,
  "risk_level": "low",
  "reason": "allowed_read_operation",
} if {
  input.tool == "argocd"
  input.operation == "app-list"
}

decision := {
  "allow": true,
  "risk_level": "low",
  "reason": "allowed_read_operation",
} if {
  input.tool == "argocd"
  input.operation == "app-get"
}

decision := {
  "allow": true,
  "risk_level": "high",
  "reason": "allowed_write_dev",
} if {
  input.tool == "argocd"
  input.operation == "app-sync"
  object.get(input.context, "environment", "") == "dev"
}

decision := {
  "allow": true,
  "risk_level": "high",
  "reason": "allowed_write_dev",
} if {
  input.tool == "argocd"
  input.operation == "app-rollback"
  object.get(input.context, "environment", "") == "dev"
}

decision := {
  "allow": true,
  "risk_level": "critical",
  "reason": "allowed_write_prod",
} if {
  input.tool == "argocd"
  input.operation == "app-sync"
  object.get(input.context, "environment", "") == "prod"
}

decision := {
  "allow": true,
  "risk_level": "critical",
  "reason": "allowed_write_prod",
} if {
  input.tool == "argocd"
  input.operation == "app-rollback"
  object.get(input.context, "environment", "") == "prod"
}
