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
    ]
  }
  not d.allow
  d.risk_level == "high"
  count(d.reasons) == 1
  d.reason == "Changes in kube-system require breakglass"
  "POL-KUBE-01" in d.hits
  "Add risk_tag: breakglass" in d.hints
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
    ]
  }
  d.allow
  d.risk_level == "normal"
  d.reason == "ok"
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
    ]
  }
 not d.allow
 count(d.hints) == 2
  some idx1
  d.hints[idx1] == "Reduce deletion scope"
  some idx2
  d.hints[idx2] == "Or add risk_tag: breakglass"
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
    ]
  }
  d.allow
  d.risk_level == "high"
  d.reason == "ok"
}
