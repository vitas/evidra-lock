package evidra.policy

import rego.v1

test_deny_kube_system_without_breakglass if {
  payload := {
    "tool": "kubectl",
    "operation": "apply",
    "context": {"environment": "dev"},
    "actions": [
      {
        "kind": "k8s.apply",
        "target": "kube-system",
        "risk_tags": [],
        "payload": {"namespace": "kube-system"}
      }
    ]
  }
  d := data.evidra.policy.decision with input as payload
  not d.allow
  d.reason == "Changes in kube-system require breakglass"
  "POL-KUBE-01" in d.hits
  "Add risk_tag: breakglass" in d.hints
}
