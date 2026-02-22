package evidra.policy.defaults

has_tag(a, tag) if {
  tags := object.get(a, "risk_tags", [])
  tags[_] == tag
}

action_namespace(a) := ns if {
  payload := object.get(a, "payload", {})
  ns := object.get(payload, "namespace", "")
  ns != ""
}

action_namespace(a) := "" if {
  payload := object.get(a, "payload", {})
  ns := object.get(payload, "namespace", "")
  ns == ""
}
