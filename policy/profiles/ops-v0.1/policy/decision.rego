package evidra.policy.decision_impl

default allow := true

denies := [ {"label": l, "message": m} | data.evidra.policy.deny[l] = m ]
warnings := [ {"label": l, "message": m} | data.evidra.policy.warn[l] = m ]

allow := count(denies) == 0
reasons := [entry.message | entry := denies[_]]
reason := reasons[0] if {
  count(reasons) > 0
}
reason := "ok" if {
  count(reasons) == 0
}

hit_labels := [entry.label | entry := denies[_]]
warn_labels := [entry.label | entry := warnings[_]]

hits := dedupe(array.concat(hit_labels, warn_labels))
hints := dedupe([hint |
  label := hits[_]
  hs := data.rule_hints[label]
  hint := hs[_]
])

risk_level := "high" if {
  count(denies) > 0
}
risk_level := "high" if {
  count(denies) == 0
  has_any_risk_tag("breakglass")
}
risk_level := "normal" if {
  count(denies) == 0
  not has_any_risk_tag("breakglass")
}

decision := {
  "allow": allow,
  "risk_level": risk_level,
  "reason": reason,
  "reasons": reasons,
  "hits": hits,
  "hints": hints,
}

has_any_risk_tag(tag) if {
  some i
  action := input.actions[i]
  tags := object.get(action, "risk_tags", [])
  tags[_] == tag
}

dedupe(xs) := sorted if {
  set := {x | x := xs[_]}
  sorted := sort([v | v := set[_]])
}
