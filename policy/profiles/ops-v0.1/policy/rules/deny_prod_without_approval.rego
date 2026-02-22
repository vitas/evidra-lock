package evidra.policy.rules

deny["prod-requires-approval"] = "prod namespace requires change-approved tag" if {
  some action in actions
  action_namespace(action) == "prod"
  not has_tag(action, "change-approved")
}
