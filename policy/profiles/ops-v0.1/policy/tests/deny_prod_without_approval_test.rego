package evidra.policy

import rego.v1

test_deny_prod_without_change_approved if {
  payload := {
    "tool": "kubectl",
    "operation": "apply",
    "context": {"environment": "prod"},
    "actions": [
      {
        "kind": "k8s.apply",
        "target": "prod",
        "risk_tags": [],
        "payload": {"namespace": "prod"}
      }
    ]
  }
  d := data.evidra.policy.decision with input as payload
  not d.allow
  d.reason == "Production changes require change-approved"
  "POL-PROD-01" in d.hits
}
