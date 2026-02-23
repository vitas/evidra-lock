package evidra.policy

import rego.v1

test_deny_mass_delete_without_breakglass if {
  payload := {
    "tool": "kubectl",
    "operation": "delete",
    "context": {"environment": "dev"},
    "actions": [
      {
        "kind": "kubectl.delete",
        "target": "default",
        "risk_tags": [],
        "payload": {"resource_count": 12}
      }
    ]
  }
  d := data.evidra.policy.decision with input as payload
  not d.allow
  d.reason == "Mass delete actions exceed threshold"
  "POL-DEL-01" in d.hits
}

test_deny_mass_destroy_without_breakglass if {
  payload := {
    "tool": "terraform",
    "operation": "plan",
    "context": {"environment": "dev"},
    "actions": [
      {
        "kind": "terraform.plan",
        "target": "infra",
        "risk_tags": [],
        "payload": {"destroy_count": 12}
      }
    ]
  }
  d := data.evidra.policy.decision with input as payload
  not d.allow
  d.reason == "Mass delete actions exceed threshold"
  "POL-DEL-01" in d.hits
}
