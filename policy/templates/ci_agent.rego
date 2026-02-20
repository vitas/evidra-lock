package evidra
import rego.v1

default ci_agent_allow := false
default ci_agent_reason := data.evidra.reason_codes.policy_denied_default
default ci_agent_risk_level := data.evidra.reason_codes.risk_critical

high_risk_ops := {op | op := data.high_risk_operations[_]}

ci_agent_allow if {
	input.actor.type == "ai"
	input.tool == "echo"
	input.operation == "run"
}

ci_agent_allow if {
	input.actor.type == "ai"
	input.tool == "git"
	input.operation == "status"
}

ci_agent_reason := data.evidra.reason_codes.allowed_by_rule if {
	ci_agent_allow
}

ci_agent_reason := data.evidra.reason_codes.policy_denied_high_risk if {
	input.operation in high_risk_ops
}

ci_agent_risk_level := data.evidra.reason_codes.risk_low if {
	ci_agent_allow
}

ci_agent_risk_level := data.evidra.reason_codes.risk_high if {
	input.operation in high_risk_ops
}

ci_agent_decision := {
	"allow": ci_agent_allow,
	"risk_level": ci_agent_risk_level,
	"reason": ci_agent_reason,
}
