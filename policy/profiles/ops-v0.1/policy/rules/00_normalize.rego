package evidra.policy.rules

actions := v {
  v := object.get(input, "actions", [])
  is_array(v)
}

default actions := []

has_tag(action, tag) {
  some t in object.get(action, "risk_tags", [])
  lower(t) == lower(tag)
}

action_kind(action) := lower(object.get(action, "kind", ""))

action_namespace(action) := lower(object.get(object.get(action, "payload", {}), "namespace", "")) if {
  object.get(object.get(action, "payload", {}), "namespace", "") != ""
}

action_namespace(action) := lower(object.get(action, "target", "")) if {
  object.get(action, "target", "") != ""
  object.get(object.get(action, "payload", {}), "namespace", "") == ""
}

action_payload_number(action, field) := num {
  raw := object.get(object.get(action, "payload", {}), field, 0)
  num := to_number(raw)
}

action_payload_bool(action, field) := object.get(object.get(action, "payload", {}), field, false)

actor_type := lower(object.get(object.get(input, "actor", {}), "type", ""))

input_source := lower(object.get(input, "source", lower(object.get(object.get(input, "actor", {}), "origin", ""))))
