package evidra.policy

import rego.v1

test_deny_public_exposure_without_approval if {
  payload := {
    "tool": "terraform",
    "operation": "plan",
    "context": {"environment": "dev"},
    "actions": [
      {
        "kind": "terraform.plan",
        "target": "infra",
        "risk_tags": [],
        "payload": {"publicly_exposed": true}
      }
    ]
  }
  d := data.evidra.policy.decision with input as payload
  not d.allow
  d.reason == "Public exposure requires approved_public"
  "POL-PUB-01" in d.hits
}
