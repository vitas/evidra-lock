package evidra.policy

import data.evidra.policy.defaults as defaults

deny["POL-PROD-01"] = msg if {
  some i
  action := input.actions[i]
  defaults.action_namespace(action) == "prod"
  not defaults.has_tag(action, "change-approved")
  msg := "Production changes require change-approved"
}
