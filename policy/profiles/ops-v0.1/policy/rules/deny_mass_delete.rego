package evidra.policy.rules

mass_delete_limit := object.get(object.get(data, "thresholds", {}), "mass_delete_max", 5)

deny["mass-delete"] = "mass delete requires breakglass" {
  some action in actions
  action_kind(action) == "kubectl.delete"
  action_payload_number(action, "resource_count") > mass_delete_limit
  not has_tag(action, "breakglass")
}

deny["mass-delete"] = "mass delete requires breakglass" {
  some action in actions
  action_kind(action) == "terraform.plan"
  action_payload_number(action, "destroy_count") > mass_delete_limit
  not has_tag(action, "breakglass")
}
