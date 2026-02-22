package evidra.policy

import rego.v1

test_deny_kube_system_without_breakglass if {
  d := decision with input as {
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
    ,
    "policy_data": policy_test_data
  }
  not d.allow
  d.reason == "kube-system changes require breakglass"
  "kube-system-breakglass" in d.hits
  "Add risk_tags=[\"breakglass\"] for controlled kube-system changes." in d.hints
}
