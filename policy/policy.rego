package evidra.policy
import rego.v1

allow := true if {
	input.tool == "echo"
	input.operation == "run"
} else := true if {
	input.tool == "git"
	input.operation == "status"
} else := false

reason := "allowed_by_rule" if {
	input.tool == "echo"
	input.operation == "run"
} else := "allowed_by_rule" if {
	input.tool == "git"
	input.operation == "status"
} else := "policy_denied_default"

risk_level := "low" if {
	allow
} else := "critical"

decision := {"allow": allow, "risk_level": risk_level, "reason": reason}
