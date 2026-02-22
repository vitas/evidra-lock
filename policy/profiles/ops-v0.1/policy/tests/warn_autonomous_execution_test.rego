package evidra.policy

import rego.v1

test_warn_autonomous_execution_high_risk if {
  d := decision with input as {
    "tool": "cmd",
    "operation": "run",
    "context": {"environment": "dev"},
    "actions": [],
    "actor": {"type": "agent", "id": "a", "origin": "mcp"},
    "source": "mcp"
  }
  d.allow
  d.risk_level == "high"
  "autonomous-execution" in d.hits
  "autonomous execution: agent via mcp" in [w.message | w := d.warnings[_]]
}
