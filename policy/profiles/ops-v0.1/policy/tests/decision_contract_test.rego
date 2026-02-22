package evidra.policy

import rego.v1

test_decision_contract if {
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
    ],
    "policy_data": policy_test_data
  }
  not d.allow
  d.risk_level == "high"
  count(d.reasons) == 1
  d.reason == "kube-system changes require breakglass"
  "kube-system-breakglass" in d.hits
  "Add risk_tags=[\"breakglass\"] for controlled kube-system changes." in d.hints
}

test_allowed_operation_reason if {
  d := decision with input as {
    "tool": "kubectl",
    "operation": "get",
    "context": {"environment": "dev"},
    "actions": [
      {
        "kind": "kubectl.get",
        "target": "default",
        "risk_tags": [],
        "payload": {"namespace": "default"}
      }
    ],
    "policy_data": policy_test_data
  }
  d.allow
  d.risk_level == "low"
  d.reason == "allowed_read_operation"
  count(d.reasons) == 0
}

test_hints_dedup if {
  d := decision with input as {
    "tool": "kubectl",
    "operation": "apply",
    "context": {"environment": "dev"},
    "actions": [
      {
        "kind": "kubectl.delete",
        "target": "default",
        "risk_tags": [],
        "payload": {"resource_count": 10, "namespace": "default"}
      },
      {
        "kind": "terraform.plan",
        "target": "default",
        "risk_tags": [],
        "payload": {"destroy_count": 8}
      }
    ],
    "policy_data": policy_test_data
  }
  not d.allow
  count(d.hints) == 1
  "Reduce delete scope below threshold or add risk_tags=[\"breakglass\"]." in d.hints
}

test_risk_high_with_breakglass_tag if {
  d := decision with input as {
    "tool": "kubectl",
    "operation": "get",
    "context": {"environment": "dev"},
    "actions": [
      {
        "kind": "kubectl.get",
        "target": "default",
        "risk_tags": ["breakglass"],
        "payload": {"namespace": "default"}
      }
    ],
    "policy_data": policy_test_data
  }
  d.allow
  d.risk_level == "high"
  d.reason == "allowed_read_operation"
}
