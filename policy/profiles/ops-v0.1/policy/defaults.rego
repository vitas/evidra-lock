package evidra.policy.defaults

import rego.v1

default raw_decision := {
  "allow": false,
  "risk_level": "critical",
  "reason": "policy_denied_default",
}

raw_decision := override_decision if {
  has_override
}

raw_decision := class_decision if {
  not has_override
}

decision := {
  "allow": object.get(raw_decision, "allow", false),
  "risk_level": object.get(raw_decision, "risk_level", "critical"),
  "reason": object.get(raw_decision, "reason", "policy_denied_default"),
}

default_unknown := object.get(data.defaults, "unknown_operation", {
  "allow": false,
  "risk_level": "critical",
  "reason": "policy_denied_default",
})

class_decision := default_unknown if {
  op_class == "unknown"
}

class_decision := object.get(data.defaults, "read", default_unknown) if {
  op_class == "read"
}

class_decision := object.get(data.defaults, "write_dev", default_unknown) if {
  op_class == "write"
  env_kind == "dev"
}

class_decision := object.get(data.defaults, "write_prod", default_unknown) if {
  op_class == "write"
  env_kind == "prod"
}

class_decision := object.get(data.defaults, "destructive_dev", default_unknown) if {
  op_class == "destructive"
  env_kind == "dev"
}

class_decision := object.get(data.defaults, "destructive_prod", default_unknown) if {
  op_class == "destructive"
  env_kind == "prod"
}

has_override if {
  matched_override
}

override_decision := {
  "allow": object.get(matched_override, "allow", object.get(default_unknown, "allow", false)),
  "risk_level": object.get(matched_override, "risk_level", object.get(default_unknown, "risk_level", "critical")),
  "reason": object.get(matched_override, "reason", object.get(default_unknown, "reason", "policy_denied_default")),
}

matched_override := ov if {
  some ov in data.overrides
  object.get(ov, "tool", "") == input.tool
  object.get(ov, "operation", "") == input.operation
  env_match(ov)
}

env_match(ov) if {
  object.get(ov, "environment", "") == ""
}

env_match(ov) if {
  object.get(ov, "environment", "") == env_kind
}

env_kind := "prod" if {
  is_prod_env
}

env_kind := "dev" if {
  not is_prod_env
  is_dev_env
}

env_kind := "prod" if {
  not is_prod_env
  not is_dev_env
}

env_value := lower(object.get(input.context, "environment", ""))

op_class := "read" if {
  is_read_op
}

op_class := "write" if {
  not is_read_op
  is_write_op
}

op_class := "destructive" if {
  not is_read_op
  not is_write_op
  is_destructive_op
}

op_class := "unknown" if {
  not is_read_op
  not is_write_op
  not is_destructive_op
}

op_key := sprintf("%s/%s", [input.tool, input.operation])

is_prod_env if {
  some e in data.environments.prod
  lower(e) == env_value
}

is_dev_env if {
  some e in data.environments.dev
  lower(e) == env_value
}

is_read_op if {
  some v in data.operation_classes.read
  v == op_key
}

is_write_op if {
  some v in data.operation_classes.write
  v == op_key
}

is_destructive_op if {
  some v in data.operation_classes.destructive
  v == op_key
}
