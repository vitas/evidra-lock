package evidra
import rego.v1

default dev_safe_allow := false
default dev_safe_reason := data.evidra.reason_codes.policy_denied_default
default dev_safe_risk_level := data.evidra.reason_codes.risk_critical

high_risk_ops := {"push", "commit", "rm", "delete", "remove"}

dev_safe_allow if {
	input.tool == "echo"
	input.operation == "run"
}

dev_safe_allow if {
	input.tool == "git"
	input.operation == "status"
}

dev_safe_reason := data.evidra.reason_codes.allowed_by_rule if {
	dev_safe_allow
}

dev_safe_reason := data.evidra.reason_codes.policy_denied_high_risk if {
	input.operation in high_risk_ops
}

dev_safe_risk_level := data.evidra.reason_codes.risk_low if {
	dev_safe_allow
}

dev_safe_risk_level := data.evidra.reason_codes.risk_high if {
	input.operation in high_risk_ops
}

dev_safe_decision := {
	"allow": dev_safe_allow,
	"risk_level": dev_safe_risk_level,
	"reason": dev_safe_reason,
}
