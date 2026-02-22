package evidra.policy

import data.evidra.policy.defaults as defaults

deny["POL-DEL-01"] = "Mass delete actions exceed threshold" if {
  mass_violation_exists
}

mass_violation_exists if {
  threshold := data.thresholds.mass_delete_max
  action := input.actions[_]
  not defaults.has_tag(action, "breakglass")
  mass_value(action) > threshold
}

mass_value(action) := action.payload.resource_count if {
  action.kind == "kubectl.delete"
}

mass_value(action) := object.get(action.payload, "destroy_count", 0) if {
  action.kind == "terraform.plan"
}
