package evidra.policy

import rego.v1

default decision := {
  "allow": false,
  "risk_level": "critical",
  "reason": "policy_denied_default",
}

decision := {"allow": true, "risk_level": "low", "reason": "allowed_read_operation"} if {
  input.tool == "docker-compose"
  input.operation == "config"
  valid_file(file)
  in_allowed_dirs(file)
}

decision := {"allow": true, "risk_level": "low", "reason": "allowed_read_operation"} if {
  input.tool == "docker-compose"
  input.operation == "ps"
  valid_file(file)
  in_allowed_dirs(file)
}

decision := {"allow": true, "risk_level": "high", "reason": "allowed_compose_write_dev"} if {
  input.tool == "docker-compose"
  input.operation == "up-service"
  env == "dev"
  valid_file(file)
  in_allowed_dirs(file)
}

decision := {"allow": true, "risk_level": "critical", "reason": "allowed_compose_write_prod"} if {
  input.tool == "docker-compose"
  input.operation == "up-service"
  env == "prod"
  valid_file(file)
  in_allowed_dirs(file)
}

decision := {"allow": true, "risk_level": "high", "reason": "allowed_compose_write_dev"} if {
  input.tool == "docker-compose"
  input.operation == "restart-service"
  env == "dev"
  valid_file(file)
  in_allowed_dirs(file)
}

decision := {"allow": true, "risk_level": "critical", "reason": "allowed_compose_write_prod"} if {
  input.tool == "docker-compose"
  input.operation == "restart-service"
  env == "prod"
  valid_file(file)
  in_allowed_dirs(file)
}

decision := {"allow": false, "risk_level": "critical", "reason": "denied_compose_file_not_allowed"} if {
  input.tool == "docker-compose"
  is_compose_op
  valid_file(file)
  not in_allowed_dirs(file)
}

decision := {"allow": false, "risk_level": "critical", "reason": "policy_denied_default"} if {
  input.tool == "docker-compose"
  is_compose_op
  not valid_file(file)
}

env := object.get(input.context, "environment", "")
file := object.get(input.params, "file", "")

valid_file(path) if {
  path != ""
  not regex.match("\\s", path)
  not contains(path, "\u0000")
}

in_allowed_dirs(path) if {
  some prefix in data.docker_compose.allowed_dirs
  startswith(path, prefix)
}

is_compose_op if input.operation == "config"
is_compose_op if input.operation == "ps"
is_compose_op if input.operation == "up-service"
is_compose_op if input.operation == "restart-service"
