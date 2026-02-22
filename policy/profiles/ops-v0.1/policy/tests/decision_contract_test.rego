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
    ]
  }
  d.allow
  d.risk_level == "normal"
  d.reason == "allowed_operation"
  count(d.reasons) == 0
}

test_hints_dedup if {
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
      },
      {
        "kind": "kubectl.delete",
        "target": "default",
        "risk_tags": [],
        "payload": {"resource_count": 12, "namespace": "default"}
      }
    ],
    "policy_data": {
      "rule_hints": {
        "kube-system-breakglass": [
          "shared hint"
        ],
        "mass-delete": [
          "shared hint"
        ]
      }
    }
  }
  not d.allow
  count(d.hints) == 1
  "shared hint" in d.hints
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
  d.reason == "allowed_read_operation"
}
