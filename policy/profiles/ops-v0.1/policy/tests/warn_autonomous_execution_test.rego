package evidra.policy

import rego.v1

test_warn_autonomous_execution_collects_hits_and_hints if {
  payload := {
    "tool": "kubectl",
    "operation": "apply",
    "context": {"environment": "dev"},
    "source": "mcp",
    "actor": {"type": "agent"},
    "actions": [
      {
        "kind": "kubectl.apply",
        "target": "default",
        "risk_tags": [],
        "payload": {"namespace": "default"}
      }
    ]
  }
  d := data.evidra.policy.decision with input as payload
  d.allow
  d.risk_level == "normal"
  "WARN-AUTO-01" in d.hits
  "Review changes manually before apply" in d.hints
  d.reason == "ok"
}
