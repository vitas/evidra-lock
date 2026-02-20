package evidra
import rego.v1

test_regulated_dev_allowed_human_git_status if {
	decision := data.evidra.regulated_dev_decision with input as {
		"actor": {"type": "human", "id": "u1", "origin": "cli"},
		"tool": "git",
		"operation": "status",
		"params": {},
		"context": {}
	}
	decision.allow == true
	decision.reason == data.evidra.reason_codes.allowed_by_rule
	decision.risk_level == data.evidra.reason_codes.risk_low
}

test_regulated_dev_denied_ai_git_status if {
	decision := data.evidra.regulated_dev_decision with input as {
		"actor": {"type": "ai", "id": "agent-1", "origin": "api"},
		"tool": "git",
		"operation": "status",
		"params": {},
		"context": {}
	}
	decision.allow == false
	decision.reason == data.evidra.reason_codes.policy_denied_default
	decision.risk_level == data.evidra.reason_codes.risk_critical
}

test_regulated_dev_denied_high_risk if {
	decision := data.evidra.regulated_dev_decision with input as {
		"actor": {"type": "human", "id": "u1", "origin": "cli"},
		"tool": "git",
		"operation": "commit",
		"params": {},
		"context": {}
	}
	decision.allow == false
	decision.reason == data.evidra.reason_codes.policy_denied_high_risk
	decision.risk_level == data.evidra.reason_codes.risk_high
}
