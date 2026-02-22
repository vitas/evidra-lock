package evidra.policy

import data.evidra.policy.defaults as defaults

deny["POL-PUB-01"] = msg if {
  some i
  action := input.actions[i]
  action.kind == "terraform.plan"
  action.payload.publicly_exposed == true
  not defaults.has_tag(action, "approved_public")
  msg := "Public exposure requires approved_public"
}
