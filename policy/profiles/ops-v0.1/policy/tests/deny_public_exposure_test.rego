package evidra.policy

import rego.v1

test_deny_public_exposure_without_approval if {
  d := decision with input as {
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
    ],
    "policy_data": policy_test_data
  }
  not d.allow
  d.reason == "public exposure requires approved_public tag"
  "public-exposure-requires-approval" in d.hits
}
