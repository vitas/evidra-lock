package evidra.policy

import rego.v1

default decision := {
  "allow": false,
  "risk_level": "critical",
  "reason": "policy_denied_default",
}

decision := {"allow": true, "risk_level": "low", "reason": "allowed_read_operation"} if {
  input.tool == "aws"
  input.operation == "s3-ls"
  valid_uri
}

decision := {"allow": true, "risk_level": "high", "reason": "allowed_s3_delete_object_dev"} if {
  input.tool == "aws"
  input.operation == "s3-rm-object"
  valid_uri
  env == "dev"
  starts_with_allow_prefix(object.get(input.params, "uri", ""), data.aws_s3.allowed_delete_prefixes)
}

decision := {"allow": true, "risk_level": "critical", "reason": "allowed_s3_delete_object_prod"} if {
  input.tool == "aws"
  input.operation == "s3-rm-object"
  valid_uri
  env == "prod"
  starts_with_allow_prefix(object.get(input.params, "uri", ""), data.aws_s3.allowed_delete_prefixes)
}

decision := {"allow": true, "risk_level": "critical", "reason": "allowed_s3_delete_recursive_dev"} if {
  input.tool == "aws"
  input.operation == "s3-rm-recursive"
  valid_uri
  env == "dev"
  starts_with_allow_prefix(object.get(input.params, "uri", ""), data.aws_s3.allowed_recursive_prefixes)
}

decision := {"allow": false, "risk_level": "critical", "reason": "denied_s3_delete_recursive_prod"} if {
  input.tool == "aws"
  input.operation == "s3-rm-recursive"
  valid_uri
  env == "prod"
}

decision := {"allow": false, "risk_level": "critical", "reason": "policy_denied_high_risk"} if {
  input.tool == "aws"
  input.operation == "s3-rm-object"
  valid_uri
  not starts_with_allow_prefix(object.get(input.params, "uri", ""), data.aws_s3.allowed_delete_prefixes)
}

decision := {"allow": false, "risk_level": "critical", "reason": "policy_denied_high_risk"} if {
  input.tool == "aws"
  input.operation == "s3-rm-recursive"
  valid_uri
  env == "dev"
  not starts_with_allow_prefix(object.get(input.params, "uri", ""), data.aws_s3.allowed_recursive_prefixes)
}

decision := {"allow": false, "risk_level": "critical", "reason": "policy_denied_default"} if {
  input.tool == "aws"
  input.operation == "s3-ls"
  not valid_uri
}

decision := {"allow": false, "risk_level": "critical", "reason": "policy_denied_default"} if {
  input.tool == "aws"
  input.operation == "s3-rm-object"
  not valid_uri
}

decision := {"allow": false, "risk_level": "critical", "reason": "policy_denied_default"} if {
  input.tool == "aws"
  input.operation == "s3-rm-recursive"
  not valid_uri
}

env := object.get(input.context, "environment", "")

valid_uri if {
  uri := object.get(input.params, "uri", "")
  startswith(uri, "s3://")
  tail := trim_prefix(uri, "s3://")
  tail != ""
  not regex.match("\\s", uri)
  parts := split(tail, "/")
  count(parts) > 0
  parts[0] != ""
}

starts_with_allow_prefix(uri, prefixes) if {
  some p in prefixes
  startswith(uri, p)
}
