package evidra.policy

import rego.v1

default decision := {
  "allow": false,
  "risk_level": "critical",
  "reason": "policy_denied_default",
}

# kubectl
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

# helm
decision := {"allow": true, "risk_level": "low", "reason": "allowed_read_operation"} if {
  input.tool == "helm"
  input.operation == "version"
}
decision := {"allow": true, "risk_level": "low", "reason": "allowed_read_operation"} if {
  input.tool == "helm"
  input.operation == "list"
}
decision := {"allow": true, "risk_level": "low", "reason": "allowed_read_operation"} if {
  input.tool == "helm"
  input.operation == "status"
}
decision := {"allow": true, "risk_level": "high", "reason": "allowed_write_dev"} if {
  input.tool == "helm"
  input.operation == "upgrade"
  object.get(input.context, "environment", "") == "dev"
}
decision := {"allow": true, "risk_level": "critical", "reason": "allowed_write_prod"} if {
  input.tool == "helm"
  input.operation == "upgrade"
  object.get(input.context, "environment", "") == "prod"
}

# terraform
decision := {"allow": true, "risk_level": "medium", "reason": "allowed_read_or_prepare"} if {
  input.tool == "terraform"
  input.operation == "version"
}
decision := {"allow": true, "risk_level": "medium", "reason": "allowed_read_or_prepare"} if {
  input.tool == "terraform"
  input.operation == "init"
}
decision := {"allow": true, "risk_level": "medium", "reason": "allowed_read_or_prepare"} if {
  input.tool == "terraform"
  input.operation == "plan"
}
decision := {"allow": true, "risk_level": "high", "reason": "allowed_apply_dev"} if {
  input.tool == "terraform"
  input.operation == "apply"
  object.get(input.context, "environment", "") == "dev"
}
decision := {"allow": true, "risk_level": "critical", "reason": "allowed_apply_prod"} if {
  input.tool == "terraform"
  input.operation == "apply"
  object.get(input.context, "environment", "") == "prod"
}

# argocd
decision := {"allow": true, "risk_level": "low", "reason": "allowed_read_operation"} if {
  input.tool == "argocd"
  input.operation == "version"
}
decision := {"allow": true, "risk_level": "low", "reason": "allowed_read_operation"} if {
  input.tool == "argocd"
  input.operation == "app-list"
}
decision := {"allow": true, "risk_level": "low", "reason": "allowed_read_operation"} if {
  input.tool == "argocd"
  input.operation == "app-get"
}
decision := {"allow": true, "risk_level": "high", "reason": "allowed_write_dev"} if {
  input.tool == "argocd"
  input.operation == "app-sync"
  object.get(input.context, "environment", "") == "dev"
}
decision := {"allow": true, "risk_level": "high", "reason": "allowed_write_dev"} if {
  input.tool == "argocd"
  input.operation == "app-rollback"
  object.get(input.context, "environment", "") == "dev"
}
decision := {"allow": true, "risk_level": "critical", "reason": "allowed_write_prod"} if {
  input.tool == "argocd"
  input.operation == "app-sync"
  object.get(input.context, "environment", "") == "prod"
}
decision := {"allow": true, "risk_level": "critical", "reason": "allowed_write_prod"} if {
  input.tool == "argocd"
  input.operation == "app-rollback"
  object.get(input.context, "environment", "") == "prod"
}

