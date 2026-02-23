package evidra.policy

import rego.v1

test_warn_breakglass_exposure_lists_hits_and_hints if {
  d := decision with input as {
    "tool": "kubectl",
    "operation": "apply",
    "context": {"environment": "prod"},
    "source": "cli",
    "actor": {"type": "human"},
    "actions": [
      {
        "kind": "kubectl.apply",
        "target": "kube-system",
        "risk_tags": ["breakglass"],
        "payload": {"namespace": "kube-system"}
      }
    ]
  }
  d.allow
  "WARN-BREAKGLASS-01" in d.hits
  "Use breakglass only for emergencies and ensure thorough review." in d.hints
  d.reason == "breakglass tag present"
}
