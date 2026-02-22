package evidra.policy.rules

deny["public-exposure-requires-approval"] = "public exposure requires approved_public tag" if {
  some action in actions
  action_kind(action) == "terraform.plan"
  action_payload_bool(action, "publicly_exposed")
  not has_tag(action, "approved_public")
}
