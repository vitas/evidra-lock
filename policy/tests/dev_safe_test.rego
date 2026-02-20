package evidra
import rego.v1

test_dev_safe_allowed_echo_run if {
	decision := data.evidra.dev_safe_decision with input as {
		"actor": {"type": "human", "id": "u1", "origin": "cli"},
		"tool": "echo",
		"operation": "run",
		"params": {"text": "ok"},
		"context": {}
	}
	decision.allow == true
	decision.reason == data.evidra.reason_codes.allowed_by_rule
	decision.risk_level == data.evidra.reason_codes.risk_low
}

test_dev_safe_denied_default if {
	decision := data.evidra.dev_safe_decision with input as {
		"actor": {"type": "human", "id": "u1", "origin": "cli"},
		"tool": "git",
		"operation": "pull",
		"params": {},
		"context": {}
	}
	decision.allow == false
	decision.reason == data.evidra.reason_codes.policy_denied_default
	decision.risk_level == data.evidra.reason_codes.risk_critical
}

test_dev_safe_denied_high_risk if {
	decision := data.evidra.dev_safe_decision with input as {
		"actor": {"type": "human", "id": "u1", "origin": "cli"},
		"tool": "git",
		"operation": "push",
		"params": {},
		"context": {}
	}
	decision.allow == false
	decision.reason == data.evidra.reason_codes.policy_denied_high_risk
	decision.risk_level == data.evidra.reason_codes.risk_high
}
