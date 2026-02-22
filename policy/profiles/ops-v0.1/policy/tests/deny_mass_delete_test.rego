package evidra.policy

import rego.v1

test_deny_mass_delete_without_breakglass if {
  d := decision with input as {
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
  not d.allow
  d.reason == "mass delete requires breakglass"
  "mass-delete" in d.hits
}
