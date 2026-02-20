package evidra
import rego.v1

default regulated_dev_allow := false
default regulated_dev_reason := data.evidra.reason_codes.policy_denied_default
default regulated_dev_risk_level := data.evidra.reason_codes.risk_critical

write_like_ops := {"push", "commit", "rm", "delete", "remove"}

regulated_dev_allow if {
	input.tool == "echo"
	input.operation == "run"
}

regulated_dev_allow if {
	input.tool == "git"
	input.operation == "status"
	input.actor.type == "human"
}

regulated_dev_reason := data.evidra.reason_codes.allowed_by_rule if {
	regulated_dev_allow
}

regulated_dev_reason := data.evidra.reason_codes.policy_denied_high_risk if {
	input.operation in write_like_ops
}

regulated_dev_reason := data.evidra.reason_codes.policy_denied_default if {
	input.tool == "git"
	input.operation == "status"
	input.actor.type == "ai"
}

regulated_dev_risk_level := data.evidra.reason_codes.risk_low if {
	regulated_dev_allow
}

regulated_dev_risk_level := data.evidra.reason_codes.risk_high if {
	input.operation in write_like_ops
}

regulated_dev_decision := {
	"allow": regulated_dev_allow,
	"risk_level": regulated_dev_risk_level,
	"reason": regulated_dev_reason,
}
